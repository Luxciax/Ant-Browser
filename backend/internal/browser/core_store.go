package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ListCores 获取所有内核配置
func (m *Manager) ListCores() []Core {
	if m.CoreDAO != nil {
		cores, err := m.CoreDAO.List()
		if err == nil {
			// 同步到内存 config，供其他逻辑使用
			m.Config.Browser.Cores = cores
			return cores
		}
	}
	return m.Config.Browser.Cores
}

// SaveCore 保存内核配置（新增或更新）
func (m *Manager) SaveCore(input CoreInput) error {
	log := logger.New("Browser")
	coreId := strings.TrimSpace(input.CoreId)
	coreName := strings.TrimSpace(input.CoreName)
	corePath := strings.TrimSpace(input.CorePath)

	if coreName == "" {
		return fmt.Errorf("内核名称不能为空")
	}
	if corePath == "" {
		return fmt.Errorf("内核路径不能为空")
	}

	if m.CoreDAO != nil {
		if coreId == "" {
			coreId = uuid.NewString()
		}
		core := Core{CoreId: coreId, CoreName: coreName, CorePath: corePath, IsDefault: input.IsDefault}
		if err := m.CoreDAO.Upsert(core); err != nil {
			return err
		}
		// 同步内存
		m.syncCoresFromDAO()
		log.Info("内核配置保存", logger.F("core_id", coreId), logger.F("core_name", coreName))
		return nil
	}

	// 降级：写 config.yaml
	previousCores := append([]Core(nil), m.Config.Browser.Cores...)
	existingIndex := -1
	for i, core := range m.Config.Browser.Cores {
		if coreId != "" && strings.EqualFold(core.CoreId, coreId) {
			existingIndex = i
			break
		}
	}
	if existingIndex >= 0 {
		m.Config.Browser.Cores[existingIndex].CoreName = coreName
		m.Config.Browser.Cores[existingIndex].CorePath = corePath
		if input.IsDefault {
			m.clearDefaultCore()
			m.Config.Browser.Cores[existingIndex].IsDefault = true
		}
	} else {
		if coreId == "" {
			coreId = uuid.NewString()
		}
		newCore := Core{
			CoreId:    coreId,
			CoreName:  coreName,
			CorePath:  corePath,
			IsDefault: input.IsDefault || len(m.Config.Browser.Cores) == 0,
		}
		if newCore.IsDefault {
			m.clearDefaultCore()
		}
		m.Config.Browser.Cores = append(m.Config.Browser.Cores, newCore)
	}
	if err := m.Config.Save(m.ResolveRelativePath("config.yaml")); err != nil {
		m.Config.Browser.Cores = previousCores
		return err
	}
	log.Info("内核配置保存（文件）", logger.F("core_id", coreId))
	return nil
}

// DeleteCore 删除内核配置
func (m *Manager) DeleteCore(coreId string) error {
	log := logger.New("Browser")
	coreId = strings.TrimSpace(coreId)
	if coreId == "" {
		return fmt.Errorf("内核ID不能为空")
	}

	if m.CoreDAO != nil {
		if err := m.CoreDAO.Delete(coreId); err != nil {
			return err
		}
		m.syncCoresFromDAO()
		log.Info("内核配置删除", logger.F("core_id", coreId))
		return nil
	}

	// 降级
	previousCores := append([]Core(nil), m.Config.Browser.Cores...)
	index := -1
	for i, core := range m.Config.Browser.Cores {
		if strings.EqualFold(core.CoreId, coreId) {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("内核不存在: %s", coreId)
	}
	wasDefault := m.Config.Browser.Cores[index].IsDefault
	m.Config.Browser.Cores = append(m.Config.Browser.Cores[:index], m.Config.Browser.Cores[index+1:]...)
	if wasDefault && len(m.Config.Browser.Cores) > 0 {
		m.Config.Browser.Cores[0].IsDefault = true
	}
	if err := m.Config.Save(m.ResolveRelativePath("config.yaml")); err != nil {
		m.Config.Browser.Cores = previousCores
		return err
	}
	log.Info("内核配置删除（文件）", logger.F("core_id", coreId))
	return nil
}

// SetDefaultCore 设置默认内核
func (m *Manager) SetDefaultCore(coreId string) error {
	log := logger.New("Browser")
	coreId = strings.TrimSpace(coreId)
	if coreId == "" {
		return fmt.Errorf("内核ID不能为空")
	}

	if m.CoreDAO != nil {
		if err := m.CoreDAO.SetDefault(coreId); err != nil {
			return err
		}
		m.syncCoresFromDAO()
		log.Info("设置默认内核", logger.F("core_id", coreId))
		return nil
	}

	// 降级
	previousCores := append([]Core(nil), m.Config.Browser.Cores...)
	found := false
	for i := range m.Config.Browser.Cores {
		if strings.EqualFold(m.Config.Browser.Cores[i].CoreId, coreId) {
			m.Config.Browser.Cores[i].IsDefault = true
			found = true
		} else {
			m.Config.Browser.Cores[i].IsDefault = false
		}
	}
	if !found {
		m.Config.Browser.Cores = previousCores
		return fmt.Errorf("内核不存在: %s", coreId)
	}
	if err := m.Config.Save(m.ResolveRelativePath("config.yaml")); err != nil {
		m.Config.Browser.Cores = previousCores
		return err
	}
	log.Info("设置默认内核（文件）", logger.F("core_id", coreId))
	return nil
}

// syncCoresFromDAO 从 DAO 同步内核列表到内存 config
func (m *Manager) syncCoresFromDAO() {
	if m.CoreDAO == nil {
		return
	}
	if cores, err := m.CoreDAO.List(); err == nil {
		m.Config.Browser.Cores = cores
	}
}

// clearDefaultCore 清除所有默认标记
func (m *Manager) clearDefaultCore() {
	for i := range m.Config.Browser.Cores {
		m.Config.Browser.Cores[i].IsDefault = false
	}
}
