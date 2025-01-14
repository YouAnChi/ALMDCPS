package utils

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// CleanupUploads 清理uploads目录中的旧文件
func CleanupUploads(uploadsDir string, maxAge time.Duration) {
	// 获取当前时间
	now := time.Now()

	// 遍历uploads目录
	err := filepath.Walk(uploadsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if path == uploadsDir {
			return nil
		}

		// 检查文件年龄
		if now.Sub(info.ModTime()) > maxAge {
			// 删除超过指定时间的文件
			err := os.Remove(path)
			if err != nil {
				log.Printf("删除文件失败 %s: %v", path, err)
			} else {
				log.Printf("已删除过期文件: %s", path)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("清理uploads目录时出错: %v", err)
	}
}

// StartCleanupScheduler 启动定时清理任务
func StartCleanupScheduler(uploadsDir string, maxAge time.Duration, interval time.Duration) {
	// 确保uploads目录存在
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Printf("创建uploads目录失败: %v", err)
		return
	}

	// 启动定时器
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			CleanupUploads(uploadsDir, maxAge)
		}
	}()

	log.Printf("已启动文件清理任务，清理间隔: %v, 文件最大保存时间: %v", interval, maxAge)
}
