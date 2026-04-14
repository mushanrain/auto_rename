package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config holds all configuration for the renamer service
type Config struct {
	WatchRoot          string
	EventDelay         time.Duration
	FileStableChecks   int
	FileStableInterval time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		WatchRoot:          "/home/home/openlist/file",
		EventDelay:         3 * time.Second,
		FileStableChecks:   4,
		FileStableInterval: 1 * time.Second,
	}
}

// RenamerService is the core service that watches and renames files
type RenamerService struct {
	cfg      Config
	watcher  *fsnotify.Watcher
	dirLocks sync.Map // map[string]*sync.Mutex
	watched  sync.Map // map[string]struct{}
}

// NewRenamerService creates a new renamer service
func NewRenamerService(cfg Config) (*RenamerService, error) {
	if cfg.FileStableChecks <= 0 {
		return nil, fmt.Errorf("FileStableChecks must be > 0, got %d", cfg.FileStableChecks)
	}
	if cfg.FileStableInterval <= 0 {
		return nil, fmt.Errorf("FileStableInterval must be > 0, got %s", cfg.FileStableInterval)
	}
	if cfg.EventDelay < 0 {
		return nil, fmt.Errorf("EventDelay must be >= 0, got %s", cfg.EventDelay)
	}

	rootInfo, err := os.Stat(cfg.WatchRoot)
	if err != nil {
		return nil, fmt.Errorf("watch root check failed: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("watch root is not a directory: %s", cfg.WatchRoot)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher failed: %w", err)
	}

	s := &RenamerService{
		cfg:     cfg,
		watcher: watcher,
	}
	return s, nil
}

// Close closes the watcher
func (s *RenamerService) Close() error {
	if s.watcher == nil {
		return nil
	}
	return s.watcher.Close()
}

// Start starts the renamer service
func (s *RenamerService) Start(ctx context.Context) error {
	if err := s.addWatchDir(s.cfg.WatchRoot); err != nil {
		return err
	}

	if err := s.watchExistingFirstLevelDirs(); err != nil {
		return err
	}

	log.Printf("service started, watch root: %s", s.cfg.WatchRoot)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("watcher error: %v", err)
		case event, ok := <-s.watcher.Events:
			if !ok {
				return nil
			}
			if !s.shouldHandleEvent(event.Op) {
				continue
			}
			log.Printf("file event: op=%s path=%s", event.Op.String(), event.Name)
			go s.handleEventPath(event.Name)
		}
	}
}

// RunWithSignal handles signals and runs the service
func RunWithSignal(service *RenamerService) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal: %s, shutting down", sig.String())
		cancel()
	}()

	if err := service.Start(ctx); err != nil {
		log.Fatalf("service stopped with error: %v", err)
	}

	log.Printf("service stopped")
}

func (s *RenamerService) shouldHandleEvent(op fsnotify.Op) bool {
	return (op&fsnotify.Create) != 0 || (op&fsnotify.Rename) != 0
}

func (s *RenamerService) watchExistingFirstLevelDirs() error {
	entries, err := os.ReadDir(s.cfg.WatchRoot)
	if err != nil {
		return fmt.Errorf("read watch root failed: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(s.cfg.WatchRoot, entry.Name())
		if err := s.addWatchDir(subDir); err != nil {
			return err
		}
	}
	return nil
}

func (s *RenamerService) addWatchDir(dir string) error {
	dir = filepath.Clean(dir)
	if _, loaded := s.watched.LoadOrStore(dir, struct{}{}); loaded {
		return nil
	}
	if err := s.watcher.Add(dir); err != nil {
		s.watched.Delete(dir)
		return fmt.Errorf("add watch failed for %s: %w", dir, err)
	}
	log.Printf("watching directory: %s", dir)
	return nil
}

func (s *RenamerService) handleEventPath(path string) {
	path = filepath.Clean(path)

	// First handle dynamic first-level directory creation.
	s.tryAddNewFirstLevelDir(path)

	parentDirName, fileName, ok := s.parseFirstLevelFile(path)
	if !ok {
		return
	}

	if shouldSkip, reason := shouldSkipFileName(fileName); shouldSkip {
		log.Printf("skip file: %s, reason: %s", path, reason)
		return
	}

	ext := filepath.Ext(fileName)
	if !isImageExt(ext) {
		log.Printf("skip file: %s, reason: non-image extension (%s)", path, ext)
		return
	}

	today := time.Now().Format("20060102")
	if isAlreadyNamed(fileName, parentDirName, today, ext) {
		log.Printf("skip file: %s, reason: already named", path)
		return
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("skip file: %s, reason: file not found", path)
			return
		}
		log.Printf("stat error: path=%s err=%v", path, err)
		return
	}
	if fileInfo.IsDir() {
		return
	}

	stable := s.waitForStableFile(path)
	if !stable {
		log.Printf("skip file: %s, reason: file is not stable", path)
		return
	}

	dirPath := filepath.Dir(path)
	mutex := s.getDirLock(dirPath)
	mutex.Lock()
	defer mutex.Unlock()

	if err := s.renameWithLock(path, parentDirName, today); err != nil {
		log.Printf("rename failed: path=%s err=%v", path, err)
	}
}

func (s *RenamerService) tryAddNewFirstLevelDir(path string) {
	rel, err := filepath.Rel(s.cfg.WatchRoot, path)
	if err != nil {
		return
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	parts := splitRelPath(rel)
	if len(parts) != 1 {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		return
	}
	if err := s.addWatchDir(path); err != nil {
		log.Printf("failed to add new subdirectory watch: path=%s err=%v", path, err)
	}
}

func (s *RenamerService) parseFirstLevelFile(path string) (parentDirName string, fileName string, ok bool) {
	rel, err := filepath.Rel(s.cfg.WatchRoot, path)
	if err != nil {
		return "", "", false
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", "", false
	}
	parts := splitRelPath(rel)
	if len(parts) != 2 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func splitRelPath(rel string) []string {
	cleaned := filepath.Clean(rel)
	if cleaned == "." {
		return nil
	}
	return strings.Split(cleaned, string(filepath.Separator))
}

func shouldSkipFileName(name string) (bool, string) {
	if name == "" {
		return true, "empty name"
	}

	if strings.HasPrefix(name, ".") {
		if strings.EqualFold(name, ".DS_Store") {
			return true, "system file .DS_Store"
		}
		return true, "hidden file"
	}
	if strings.HasPrefix(name, "~$") {
		return true, "temporary prefix ~$"
	}
	if strings.HasPrefix(name, "._") {
		return true, "temporary prefix ._"
	}

	ext := strings.ToLower(filepath.Ext(name))
	tmpExts := map[string]struct{}{
		".tmp":        {},
		".temp":       {},
		".part":       {},
		".crdownload": {},
		".download":   {},
		".partial":    {},
		".filepart":   {},
		".swp":        {},
		".swo":        {},
	}
	if _, ok := tmpExts[ext]; ok {
		return true, fmt.Sprintf("temporary extension (%s)", ext)
	}

	return false, ""
}

func isImageExt(ext string) bool {
	imageExts := map[string]struct{}{
		".jpg":  {},
		".jpeg": {},
		".png":  {},
		".webp": {},
		".gif":  {},
		".bmp":  {},
		".tiff": {},
		".heic": {},
		".heif": {},
	}
	_, ok := imageExts[strings.ToLower(ext)]
	return ok
}

func isAlreadyNamed(fileName, parentDirName, date, ext string) bool {
	base := strings.TrimSuffix(fileName, ext)
	prefix := parentDirName + date + "-"
	if !strings.HasPrefix(base, prefix) {
		return false
	}
	numPart := strings.TrimPrefix(base, prefix)
	if numPart == "" {
		return false
	}
	n, err := strconv.Atoi(numPart)
	if err != nil {
		return false
	}
	return n > 0
}

func (s *RenamerService) waitForStableFile(path string) bool {
	if s.cfg.EventDelay > 0 {
		time.Sleep(s.cfg.EventDelay)
	}

	var lastSize int64 = -1
	stableCount := 0
	maxRounds := s.cfg.FileStableChecks * 5

	for i := 0; i < maxRounds; i++ {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false
			}
			log.Printf("stable check stat error: path=%s err=%v", path, err)
			return false
		}
		if info.IsDir() {
			return false
		}

		size := info.Size()
		if size == lastSize {
			stableCount++
		} else {
			stableCount = 1
			lastSize = size
		}

		if stableCount >= s.cfg.FileStableChecks {
			return true
		}

		time.Sleep(s.cfg.FileStableInterval)
	}

	return false
}

func (s *RenamerService) getDirLock(dir string) *sync.Mutex {
	if v, ok := s.dirLocks.Load(dir); ok {
		return v.(*sync.Mutex)
	}
	m := &sync.Mutex{}
	actual, _ := s.dirLocks.LoadOrStore(dir, m)
	return actual.(*sync.Mutex)
}

func (s *RenamerService) renameWithLock(path, parentDirName, date string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("target is directory: %s", path)
	}

	fileName := filepath.Base(path)
	ext := filepath.Ext(fileName)
	if !isImageExt(ext) {
		log.Printf("skip file: %s, reason: non-image extension after recheck (%s)", path, ext)
		return nil
	}
	if shouldSkip, reason := shouldSkipFileName(fileName); shouldSkip {
		log.Printf("skip file: %s, reason: %s", path, reason)
		return nil
	}
	if isAlreadyNamed(fileName, parentDirName, date, ext) {
		log.Printf("skip file: %s, reason: already named", path)
		return nil
	}

	dirPath := filepath.Dir(path)
	nextSeq, err := s.nextSequence(dirPath, parentDirName, date)
	if err != nil {
		return err
	}

	for {
		newName := fmt.Sprintf("%s%s-%d%s", parentDirName, date, nextSeq, ext)
		newPath := filepath.Join(dirPath, newName)

		if sameFileName(fileName, newName) {
			log.Printf("skip file: %s, reason: already target name", path)
			return nil
		}

		if _, err := os.Stat(newPath); err == nil {
			nextSeq++
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check target path failed: %w", err)
		}

		if err := os.Rename(path, newPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("source disappeared before rename: %w", err)
			}
			return fmt.Errorf("rename error: %w", err)
		}

		log.Printf("renamed: %s -> %s", path, newPath)
		return nil
	}
}

func (s *RenamerService) nextSequence(dirPath, parentDirName, date string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, fmt.Errorf("read dir for sequence failed: %w", err)
	}

	prefix := parentDirName + date + "-"
	maxSeq := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == "" {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		if !strings.HasPrefix(base, prefix) {
			continue
		}
		numPart := strings.TrimPrefix(base, prefix)
		n, err := strconv.Atoi(numPart)
		if err != nil || n <= 0 {
			continue
		}
		if n > maxSeq {
			maxSeq = n
		}
	}
	return maxSeq + 1, nil
}

func sameFileName(a, b string) bool {
	return a == b
}
