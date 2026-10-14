package logger

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileWriterConfig 文件写入器配置
type FileWriterConfig struct {
	FilePath       string        // 日志文件路径
	BufferSize     int           // 缓冲区大小（字节），默认 4KB
	FlushInterval  time.Duration // 刷新间隔，默认 1s
	AsyncQueueSize int           // 异步队列大小，默认 1000
}

// DefaultFileWriterConfig 返回默认的文件写入器配置
func DefaultFileWriterConfig(filePath string) FileWriterConfig {
	return FileWriterConfig{
		FilePath:       filePath,
		BufferSize:     4 * 1024, // 4KB
		FlushInterval:  time.Second,
		AsyncQueueSize: 1000,
	}
}

// FileWriter 文件写入器
// 支持缓冲写入和异步写入
type FileWriter struct {
	lifecycleMu sync.RWMutex
	closed      bool
	config      FileWriterConfig
	formatter   Formatter
	file        *os.File
	buffer      *bufio.Writer
	mu          sync.Mutex

	// 异步写入相关
	asyncChan            chan *LogEntry
	done                 chan struct{}
	wg                   sync.WaitGroup
	asyncMode            bool
	flushTicker          *time.Ticker
	rotation             *RotationManager
	lastError            error
	flushRequests        chan chan error
	dropped              uint64
	lastWrite            time.Time
	lastFlush            time.Time
	lastFailure          time.Time
	bufferFailed         bool
	discardedBufferBytes uint64
	failedWrites         uint64
	lastFailureError     string
}

// NewFileWriter 创建新的文件写入器（同步模式）
func NewFileWriter(config FileWriterConfig, formatter Formatter) (*FileWriter, error) {
	if formatter == nil {
		formatter = NewTextFormatter()
	}

	// 确保目录存在
	dir := filepath.Dir(config.FilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// 打开文件（追加模式）
	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// 设置默认缓冲区大小
	bufferSize := config.BufferSize
	if bufferSize <= 0 {
		bufferSize = 4 * 1024 // 4KB
	}

	w := &FileWriter{
		config:    config,
		formatter: formatter,
		file:      file,
		buffer:    bufio.NewWriterSize(file, bufferSize),
		asyncMode: false,
	}

	return w, nil
}

// Write 写入日志条目到文件（同步模式）
func (w *FileWriter) Write(entry *LogEntry) error {
	w.lifecycleMu.RLock()
	defer w.lifecycleMu.RUnlock()
	if w.closed {
		return os.ErrClosed
	}
	if entry == nil {
		return nil
	}

	// 如果是异步模式，发送到队列
	if w.asyncMode && !criticalEntry(entry) {
		return w.writeAsync(entry)
	}

	err := w.writeSync(entry)
	w.recordError(err)
	return err
}

// writeSync 同步写入
func (w *FileWriter) writeSync(entry *LogEntry) error {
	data, err := w.formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return os.ErrClosed
	}
	if w.bufferFailed {
		w.discardedBufferBytes += uint64(w.buffer.Buffered())
		w.buffer.Reset(w.file)
		w.bufferFailed = false
	}
	if w.rotation != nil {
		info, statErr := w.file.Stat()
		if statErr != nil {
			return statErr
		}
		if w.rotation.ShouldRotate(info, entry) {
			if err := w.buffer.Flush(); err != nil {
				return err
			}
			if _, err := w.rotation.Rotate(w.file); err != nil {
				return err
			}
			file, err := os.OpenFile(w.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				return err
			}
			w.file = file
			w.buffer.Reset(file)
			resetRotationTime(w.rotation.config.Policy, entry.Timestamp)
		}
	}

	_, err = w.buffer.Write(data)
	if err != nil {
		w.bufferFailed = true
		return fmt.Errorf("failed to write to buffer: %w", err)
	}
	// A recorded crash must not wait for the next periodic flush.
	w.lastWrite = time.Now()
	if criticalEntry(entry) {
		if err := w.buffer.Flush(); err != nil {
			w.bufferFailed = true
			return err
		}
		err := w.file.Sync()
		if err == nil {
			w.lastFlush = time.Now()
		}
		return err
	}
	return nil
}

// Flush 刷新缓冲区到文件
func (w *FileWriter) Flush() error {
	w.lifecycleMu.RLock()
	defer w.lifecycleMu.RUnlock()
	if w.closed {
		return os.ErrClosed
	}
	if w.asyncMode {
		reply := make(chan error, 1)
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case w.flushRequests <- reply:
		case <-timer.C:
			return fmt.Errorf("log flush queue barrier timeout")
		}
		select {
		case err := <-reply:
			return err
		case <-timer.C:
			return fmt.Errorf("log flush persistence timeout")
		}
	}
	return w.flushBuffer()
}

func (w *FileWriter) flushBuffer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buffer != nil {
		err := w.buffer.Flush()
		if err != nil {
			w.bufferFailed = true
		}
		if err == nil {
			w.lastFlush = time.Now()
		}
		return err
	}
	return nil
}

// Close 关闭文件写入器
func (w *FileWriter) Close() error {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	// 如果是异步模式，先停止异步写入
	if w.asyncMode {
		w.stopAsync()
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	var errs []error

	// 刷新缓冲区
	if w.buffer != nil {
		if err := w.buffer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("failed to flush buffer: %w", err))
		}
	}

	// 关闭文件
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close file: %w", err))
		}
		w.file = nil
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// GetFilePath 获取当前日志文件路径
func (w *FileWriter) GetFilePath() string {
	return w.config.FilePath
}

// NewAsyncFileWriter 创建新的异步文件写入器
func NewAsyncFileWriter(config FileWriterConfig, formatter Formatter) (*FileWriter, error) {
	w, err := NewFileWriter(config, formatter)
	if err != nil {
		return nil, err
	}

	// 启用异步模式
	w.enableAsync()

	return w, nil
}

// enableAsync 启用异步写入模式
func (w *FileWriter) enableAsync() {
	if w.asyncMode {
		return
	}

	queueSize := w.config.AsyncQueueSize
	if queueSize <= 0 {
		queueSize = 1000
	}

	flushInterval := w.config.FlushInterval
	if flushInterval <= 0 {
		flushInterval = time.Second
	}

	w.asyncChan = make(chan *LogEntry, queueSize)
	w.flushRequests = make(chan chan error, 1)
	w.done = make(chan struct{})
	w.flushTicker = time.NewTicker(flushInterval)
	w.asyncMode = true

	// 启动后台写入 goroutine
	w.wg.Add(1)
	go w.asyncWriteLoop()
}

// asyncWriteLoop 异步写入循环
func (w *FileWriter) asyncWriteLoop() {
	defer w.wg.Done()

	for {
		select {
		case entry, ok := <-w.asyncChan:
			if !ok {
				// 通道已关闭，处理剩余日志
				return
			}
			// 写入日志（忽略错误，避免阻塞）
			w.recordError(w.writeSync(entry))

		case <-w.flushTicker.C:
			// 定期刷新缓冲区
			w.recordError(w.flushBuffer())

		case reply := <-w.flushRequests:
			w.drainQueue()
			err := w.flushBuffer()
			w.recordError(err)
			reply <- err

		case <-w.done:
			// 收到停止信号，处理剩余日志
			w.drainQueue()
			return
		}
	}
}

// drainQueue 清空队列中的剩余日志
func (w *FileWriter) drainQueue() {
	// Bound the barrier to entries already queued; concurrent producers cannot starve it.
	for remaining := len(w.asyncChan); remaining > 0; remaining-- {
		select {
		case entry, ok := <-w.asyncChan:
			if !ok {
				return
			}
			w.recordError(w.writeSync(entry))
		default:
			// 队列已空
			return
		}
	}
}

// writeAsync 异步写入（非阻塞）
func (w *FileWriter) writeAsync(entry *LogEntry) error {
	select {
	case w.asyncChan <- entry:
		return nil
	default:
		// 队列满，丢弃日志（非阻塞）
		w.mu.Lock()
		w.dropped++
		w.mu.Unlock()
		return ErrQueueFull
	}
}

// stopAsync 停止异步写入
func (w *FileWriter) stopAsync() {
	if !w.asyncMode {
		return
	}

	// 停止定时器
	if w.flushTicker != nil {
		w.flushTicker.Stop()
	}

	// 发送停止信号
	close(w.done)

	// 等待后台 goroutine 完成
	w.wg.Wait()

	// 关闭通道
	close(w.asyncChan)

	w.asyncMode = false
}

// IsAsync 返回是否为异步模式
func (w *FileWriter) IsAsync() bool {
	w.lifecycleMu.RLock()
	defer w.lifecycleMu.RUnlock()
	return w.asyncMode
}

// QueueLength 返回当前异步队列长度（用于监控）
func (w *FileWriter) QueueLength() int {
	w.lifecycleMu.RLock()
	defer w.lifecycleMu.RUnlock()
	if !w.asyncMode {
		return 0
	}
	return len(w.asyncChan)
}

func (w *FileWriter) recordError(err error) {
	w.mu.Lock()
	w.lastError = err
	if err != nil {
		w.lastFailure = time.Now()
		w.failedWrites++
		w.lastFailureError = RedactText(err.Error())
	}
	w.mu.Unlock()
}

var ErrQueueFull = errors.New("async queue full, ordinary log entry dropped")

func Health() map[string]interface{} {
	globalMu.RLock()
	defer globalMu.RUnlock()
	out := map[string]interface{}{"app_session_id": evidenceSession, "dropped_noncritical": uint64(0)}
	if globalLogger == nil || globalLogger.fileWriter == nil {
		out["file_enabled"] = false
		return out
	}
	w := globalLogger.fileWriter
	out["file_enabled"] = true
	out["queue_length"] = w.QueueLength()
	w.mu.Lock()
	defer w.mu.Unlock()
	out["dropped_noncritical"] = w.dropped
	out["failed_write_attempts"] = w.failedWrites
	out["discarded_buffer_bytes"] = w.discardedBufferBytes
	out["last_failure_error"] = w.lastFailureError
	out["last_buffer_write_at"] = w.lastWrite.Format(time.RFC3339Nano)
	out["last_flush_at"] = w.lastFlush.Format(time.RFC3339Nano)
	out["last_failure_at"] = w.lastFailure.Format(time.RFC3339Nano)
	if w.lastError != nil {
		out["error"] = RedactText(w.lastError.Error())
	}
	return out
}

func (w *FileWriter) Error() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastError
}

func resetRotationTime(policy RotationPolicy, at time.Time) {
	switch p := policy.(type) {
	case *TimeRotationPolicy:
		p.UpdateLastRotate(at)
	case *CompositeRotationPolicy:
		for _, child := range p.GetPolicies() {
			resetRotationTime(child, at)
		}
	}
}
