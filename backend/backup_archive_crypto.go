package backend

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const encryptedBackupFormat = "ant-chrome-encrypted-full-backup"
const backupCryptoChunkSize = 1 << 20

type encryptedBackupHeader struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	Salt        string `json:"salt"`
	NoncePrefix string `json:"noncePrefix"`
	ChunkSize   int    `json:"chunkSize"`
}

func backupArchiveCipher(header encryptedBackupHeader, password string) (cipher.AEAD, []byte, []byte, error) {
	if header.Format != encryptedBackupFormat || header.Version != 1 || header.ChunkSize != backupCryptoChunkSize {
		return nil, nil, nil, fmt.Errorf("不支持的加密备份格式")
	}
	salt, err := base64.StdEncoding.DecodeString(header.Salt)
	if err != nil || len(salt) != 16 {
		return nil, nil, nil, fmt.Errorf("加密备份盐值无效")
	}
	prefix, err := base64.StdEncoding.DecodeString(header.NoncePrefix)
	if err != nil || len(prefix) != 4 {
		return nil, nil, nil, fmt.Errorf("加密备份随机值无效")
	}
	key := argon2.IDKey([]byte(password), salt, portableLoginIterations, portableLoginMemoryKiB, portableLoginParallelism, 32)
	defer zeroSensitiveBytes(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	data, _ := json.Marshal(header)
	digest := sha256.Sum256(data)
	return aead, prefix, digest[:], err
}

func backupChunkBinding(prefix, digest []byte, sequence uint64, size uint32) ([]byte, []byte) {
	nonce := make([]byte, 12)
	copy(nonce, prefix)
	binary.BigEndian.PutUint64(nonce[4:], sequence)
	aad := make([]byte, len(digest)+12)
	copy(aad, digest)
	binary.BigEndian.PutUint64(aad[len(digest):], sequence)
	binary.BigEndian.PutUint32(aad[len(digest)+8:], size)
	return nonce, aad
}

// encryptedBackupStream encrypts the inner ZIP as it is produced. No plaintext
// intermediate archive is ever written into the user's backup/sync directory.
type encryptedBackupStream struct {
	archive                *zip.Writer
	payload                io.Writer
	aead                   cipher.AEAD
	prefix, digest, buffer []byte
	used                   int
	sequence               uint64
	total                  int64
	err                    error
	finished               bool
}

func newEncryptedBackupStream(output io.Writer, password string) (*encryptedBackupStream, error) {
	if len(password) < 8 {
		return nil, fmt.Errorf("整包加密密码至少需要 8 个字符")
	}
	salt, prefix := make([]byte, 16), make([]byte, 4)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(prefix); err != nil {
		return nil, err
	}
	header := encryptedBackupHeader{encryptedBackupFormat, 1, base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(prefix), backupCryptoChunkSize}
	aead, prefix, digest, err := backupArchiveCipher(header, password)
	if err != nil {
		return nil, err
	}
	archive := zip.NewWriter(output)
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(entry).Encode(header); err != nil {
		return nil, err
	}
	entry, err = archive.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: zip.Store})
	if err != nil {
		return nil, err
	}
	return &encryptedBackupStream{archive: archive, payload: entry, aead: aead, prefix: prefix, digest: digest, buffer: make([]byte, backupCryptoChunkSize)}, nil
}

func (s *encryptedBackupStream) writeChunk(data []byte) error {
	length := uint32(len(data))
	nonce, aad := backupChunkBinding(s.prefix, s.digest, s.sequence, length)
	sealed := s.aead.Seal(nil, nonce, data, aad)
	if err := binary.Write(s.payload, binary.BigEndian, length); err != nil {
		return err
	}
	n, err := s.payload.Write(sealed)
	if err == nil && n != len(sealed) {
		err = io.ErrShortWrite
	}
	s.sequence++
	return err
}

func (s *encryptedBackupStream) Write(data []byte) (int, error) {
	if s.finished {
		return 0, fmt.Errorf("加密备份流已经结束")
	}
	if s.err != nil {
		return 0, s.err
	}
	if int64(len(data)) > profilePackageMaxCompressedBytes-s.total {
		s.err = fmt.Errorf("待加密备份超过大小限制")
		return 0, s.err
	}
	written := 0
	for len(data) > 0 {
		n := copy(s.buffer[s.used:], data)
		s.used += n
		written += n
		s.total += int64(n)
		data = data[n:]
		if s.used == len(s.buffer) {
			s.err = s.writeChunk(s.buffer)
			s.used = 0
			if s.err != nil {
				return written, s.err
			}
		}
	}
	return written, nil
}

// Every chunk authenticates its sequence/length. The mandatory authenticated
// empty terminator rejects truncation, reordering, splicing and appended bytes.
func (s *encryptedBackupStream) Finish() error {
	if s.finished {
		return fmt.Errorf("加密备份流已经结束")
	}
	s.finished = true
	defer zeroSensitiveBytes(s.buffer)
	if s.err != nil {
		return s.err
	}
	if s.used > 0 {
		if err := s.writeChunk(s.buffer[:s.used]); err != nil {
			return err
		}
	}
	if err := s.writeChunk(nil); err != nil {
		return err
	}
	return s.archive.Close()
}

func encryptFullBackupArchive(inputPath, outputPath, password string) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	stream, err := newEncryptedBackupStream(output, password)
	if err != nil {
		return err
	}
	defer zeroSensitiveBytes(stream.buffer)
	if _, err := io.Copy(stream, input); err != nil {
		return err
	}
	if err := stream.Finish(); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	return output.Close()
}

func decryptFullBackupArchive(inputPath, outputPath, password string) error {
	archive, err := zip.OpenReader(inputPath)
	if err != nil {
		return fmt.Errorf("读取加密备份失败")
	}
	defer archive.Close()
	if len(archive.File) != 2 {
		return fmt.Errorf("加密备份条目无效")
	}
	var header encryptedBackupHeader
	var payload *zip.File
	seenHeader := false
	for _, entry := range archive.File {
		switch entry.Name {
		case "manifest.json":
			if seenHeader || entry.UncompressedSize64 > 4096 {
				return fmt.Errorf("加密备份头无效")
			}
			seenHeader = true
			reader, err := entry.Open()
			if err != nil {
				return err
			}
			err = json.NewDecoder(io.LimitReader(reader, 4096)).Decode(&header)
			_ = reader.Close()
			if err != nil {
				return fmt.Errorf("加密备份头格式无效")
			}
		case "payload.bin":
			if payload != nil || entry.UncompressedSize64 > uint64(profilePackageMaxCompressedBytes)+(2<<20) {
				return fmt.Errorf("加密备份载荷无效或过大")
			}
			payload = entry
		default:
			return fmt.Errorf("加密备份包含未知条目")
		}
	}
	if !seenHeader || payload == nil {
		return fmt.Errorf("加密备份缺少必要条目")
	}
	aead, prefix, digest, err := backupArchiveCipher(header, password)
	if err != nil {
		return err
	}
	reader, err := payload.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	buffer := make([]byte, backupCryptoChunkSize+aead.Overhead())
	defer zeroSensitiveBytes(buffer)
	var total int64
	for sequence := uint64(0); ; sequence++ {
		var length uint32
		if binary.Read(reader, binary.BigEndian, &length) != nil || length > backupCryptoChunkSize {
			return fmt.Errorf("加密备份截断或分块长度无效")
		}
		sealed := buffer[:int(length)+aead.Overhead()]
		if _, err := io.ReadFull(reader, sealed); err != nil {
			return fmt.Errorf("加密备份已截断")
		}
		nonce, aad := backupChunkBinding(prefix, digest, sequence, length)
		plain, err := aead.Open(sealed[:0], nonce, sealed, aad)
		if err != nil {
			return errPortableLoginBadPassword
		}
		if length == 0 {
			var extra [1]byte
			n, err := reader.Read(extra[:])
			if n != 0 || err != io.EOF {
				return fmt.Errorf("加密备份终止记录或 ZIP 校验无效")
			}
			break
		}
		total += int64(length)
		if total > profilePackageMaxCompressedBytes {
			return fmt.Errorf("解密备份超过大小限制")
		}
		if _, err := output.Write(plain); err != nil {
			return err
		}
	}
	if err := output.Sync(); err != nil {
		return err
	}
	return output.Close()
}
