package yamlmgmt

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// HotReloadManager manages hot reloading for all YAML configurations.
type HotReloadManager struct {
	mu             sync.RWMutex
	watcher        *fsnotify.Watcher
	handlers       map[YAMLKind][]ReloadHandler
	watchDirs      map[string]YAMLKind
	versionMgr     *VersionManager
	validator      Validator
	eventChan      chan ConfigEvent
	ctx            context.Context
	cancel         context.CancelFunc
	debounceDelay  time.Duration
	pendingReloads map[string]time.Time
}

// NewHotReloadManager creates a new hot reload manager.
func NewHotReloadManager(versionMgr *VersionManager, validator Validator) (*HotReloadManager, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	hrm := &HotReloadManager{
		watcher:        watcher,
		handlers:       make(map[YAMLKind][]ReloadHandler),
		watchDirs:      make(map[string]YAMLKind),
		versionMgr:     versionMgr,
		validator:      validator,
		eventChan:      make(chan ConfigEvent, 100),
		ctx:            ctx,
		cancel:         cancel,
		debounceDelay:  500 * time.Millisecond,
		pendingReloads: make(map[string]time.Time),
	}

	// Start watching
	go hrm.watch()

	return hrm, nil
}

// RegisterHandler registers a reload handler for a specific kind.
func (hrm *HotReloadManager) RegisterHandler(kind YAMLKind, handler ReloadHandler) {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	if _, exists := hrm.handlers[kind]; !exists {
		hrm.handlers[kind] = []ReloadHandler{}
	}
	hrm.handlers[kind] = append(hrm.handlers[kind], handler)
}

// WatchDirectory starts watching a directory for YAML changes.
func (hrm *HotReloadManager) WatchDirectory(dir string, kind YAMLKind) error {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	// Ensure directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Add to watcher
	if err := hrm.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	hrm.watchDirs[dir] = kind
	log.Printf("Watching %s for %s configurations", dir, kind)

	// Load existing files
	hrm.loadExistingFiles(dir, kind)

	return nil
}

// Events returns the event channel.
func (hrm *HotReloadManager) Events() <-chan ConfigEvent {
	return hrm.eventChan
}

// Stop stops the hot reload manager.
func (hrm *HotReloadManager) Stop() {
	hrm.cancel()
	hrm.watcher.Close()
	close(hrm.eventChan)
}

// Private methods

func (hrm *HotReloadManager) watch() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("HotReloadManager panic recovered: %v", r)
		}
	}()

	debouncer := time.NewTicker(100 * time.Millisecond)
	defer debouncer.Stop()

	for {
		select {
		case <-hrm.ctx.Done():
			return

		case event, ok := <-hrm.watcher.Events:
			if !ok {
				return
			}
			hrm.handleFileEvent(event)

		case err, ok := <-hrm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
			hrm.sendEvent(ConfigEvent{
				Type:      EventTypeError,
				Timestamp: time.Now(),
				Error:     err.Error(),
			})

		case <-debouncer.C:
			hrm.processPendingReloads()
		}
	}
}

func (hrm *HotReloadManager) handleFileEvent(event fsnotify.Event) {
	// Ignore non-YAML files
	if !strings.HasSuffix(event.Name, ".yaml") && !strings.HasSuffix(event.Name, ".yml") {
		return
	}

	// Ignore hidden files and version files
	if strings.Contains(event.Name, "/.") {
		return
	}

	// Determine the kind based on directory
	kind := hrm.getKindForFile(event.Name)
	if kind == "" {
		return
	}

	// Debounce rapid changes
	hrm.mu.Lock()
	hrm.pendingReloads[event.Name] = time.Now()
	hrm.mu.Unlock()
}

func (hrm *HotReloadManager) processPendingReloads() {
	hrm.mu.Lock()
	toProcess := make(map[string]time.Time)
	now := time.Now()

	for file, timestamp := range hrm.pendingReloads {
		if now.Sub(timestamp) >= hrm.debounceDelay {
			toProcess[file] = timestamp
			delete(hrm.pendingReloads, file)
		}
	}
	hrm.mu.Unlock()

	for file := range toProcess {
		hrm.reloadFile(file)
	}
}

func (hrm *HotReloadManager) reloadFile(filename string) {
	kind := hrm.getKindForFile(filename)
	if kind == "" {
		return
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// File deleted
		name := hrm.extractNameFromFile(filename)
		hrm.sendEvent(ConfigEvent{
			Type:      EventTypeDeleted,
			Kind:      kind,
			Name:      name,
			Timestamp: time.Now(),
		})
		return
	}

	// Load the file
	doc, err := hrm.loadYAMLFile(filename)
	if err != nil {
		log.Printf("Failed to load %s: %v", filename, err)
		hrm.sendEvent(ConfigEvent{
			Type:      EventTypeError,
			Kind:      kind,
			Name:      hrm.extractNameFromFile(filename),
			Timestamp: time.Now(),
			Error:     err.Error(),
		})
		return
	}

	// Validate if validator is available
	if hrm.validator != nil {
		result, err := hrm.validator.Validate(doc)
		if err != nil || !result.Valid {
			log.Printf("Validation failed for %s: %v", filename, err)
			hrm.sendEvent(ConfigEvent{
				Type:      EventTypeError,
				Kind:      kind,
				Name:      doc.Metadata.Name,
				Timestamp: time.Now(),
				Error:     "Validation failed",
			})
			return
		}
	}

	// Create a version
	message := fmt.Sprintf("Hot reload from file change: %s", filepath.Base(filename))
	version, err := hrm.versionMgr.CreateVersion(kind, doc.Metadata.Name, doc, message)
	if err != nil {
		log.Printf("Failed to create version for %s: %v", filename, err)
		hrm.sendEvent(ConfigEvent{
			Type:      EventTypeError,
			Kind:      kind,
			Name:      doc.Metadata.Name,
			Timestamp: time.Now(),
			Error:     err.Error(),
		})
		return
	}

	// Call reload handlers
	hrm.callHandlers(kind, doc)

	// Send success event
	eventType := EventTypeModified
	if version.ParentHash == "" {
		eventType = EventTypeCreated
	}

	hrm.sendEvent(ConfigEvent{
		Type:      eventType,
		Kind:      kind,
		Name:      doc.Metadata.Name,
		Version:   version,
		Timestamp: time.Now(),
	})

	log.Printf("Successfully reloaded %s configuration: %s", kind, doc.Metadata.Name)
}

func (hrm *HotReloadManager) loadYAMLFile(filename string) (*YAMLDocument, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var doc YAMLDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Set metadata if missing
	if doc.Metadata.Name == "" {
		doc.Metadata.Name = hrm.extractNameFromFile(filename)
	}

	// Determine kind if not set
	if doc.Kind == "" {
		doc.Kind = string(hrm.getKindForFile(filename))
	}

	return &doc, nil
}

func (hrm *HotReloadManager) getKindForFile(filename string) YAMLKind {
	hrm.mu.RLock()
	defer hrm.mu.RUnlock()

	dir := filepath.Dir(filename)

	// Check exact match
	if kind, exists := hrm.watchDirs[dir]; exists {
		return kind
	}

	// Check parent directories
	for watchDir, kind := range hrm.watchDirs {
		if strings.HasPrefix(filename, watchDir) {
			return kind
		}
	}

	// Infer from path
	if strings.Contains(filename, "/routes/") {
		return KindRoute
	}
	if strings.Contains(filename, "/config/") {
		return KindConfig
	}
	if strings.Contains(filename, "/dashboards/") {
		return KindDashboard
	}
	if strings.Contains(filename, "docker-compose") {
		return KindCompose
	}

	return ""
}

func (hrm *HotReloadManager) extractNameFromFile(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func (hrm *HotReloadManager) callHandlers(kind YAMLKind, doc *YAMLDocument) {
	hrm.mu.RLock()
	handlers := hrm.handlers[kind]
	hrm.mu.RUnlock()

	for _, handler := range handlers {
		go func(h ReloadHandler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Handler panic: %v", r)
				}
			}()

			if err := h(doc); err != nil {
				log.Printf("Handler error for %s: %v", doc.Metadata.Name, err)
			}
		}(handler)
	}
}

func (hrm *HotReloadManager) sendEvent(event ConfigEvent) {
	select {
	case hrm.eventChan <- event:
	case <-time.After(100 * time.Millisecond):
		log.Printf("Event channel full, dropping event: %+v", event)
	}
}

func (hrm *HotReloadManager) loadExistingFiles(dir string, kind YAMLKind) {
	// Walk directory and load existing YAML files
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			// Load file in background
			go func() {
				time.Sleep(100 * time.Millisecond) // Small delay to avoid overwhelming
				hrm.reloadFile(path)
			}()
		}

		return nil
	})
}
