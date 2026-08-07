package config

import (
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Watcher 监听 config/indexes/*.yaml 变更（Q15 热加载）
// 变更 → 重新加载校验 → 回调（回灌 DB + 更新内存配置 + GUI 提示"需重建"）
type Watcher struct {
	watcher  *fsnotify.Watcher
	dir      string
	onChange func(name string)
	stop     chan struct{}
}

// NewWatcher 创建配置监听器；onChange 在配置变更且校验通过后回调
func NewWatcher(dir string, onChange func(name string)) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}
	return &Watcher{
		watcher:  w,
		dir:      dir,
		onChange: onChange,
		stop:     make(chan struct{}),
	}, nil
}

// Start 启动监听循环（阻塞）
func (w *Watcher) Start() {
	go func() {
		for {
			select {
			case <-w.stop:
				return
			case ev, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				// 只关心 yaml 文件的写/创建/重命名（原子写 = rename）
				if !isYaml(ev.Name) {
					continue
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				// 跳过临时文件（原子写中间态 .tmp）
				if filepath.Ext(ev.Name) == ".tmp" {
					continue
				}
				name := filepath.Base(ev.Name)
				log.Printf("[config-watch] %s changed (%v), reloading", name, ev.Op)
				if w.onChange != nil {
					w.onChange(trimYamlExt(name))
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[config-watch] error: %v", err)
			}
		}
	}()
}

// Stop 停止监听
func (w *Watcher) Stop() {
	close(w.stop)
	w.watcher.Close()
}

func isYaml(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

func trimYamlExt(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}
