package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/utils"
)

// 判断文件是否存在
func isFileExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// 创建文件夹
func createDirIfNotExist(dir string) error {
	if !isFileExist(dir) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			logger.Errorf("create dir failed: %s", err.Error())
			return err
		}
	}
	return nil
}

// saveStringToFile 保存字符串到文件
func saveStringToFile(filePath, content string) error {
	err := createDirIfNotExist(filepath.Dir(filePath))
	if err != nil {
		return err
	}
	err = os.WriteFile(filePath, []byte(content), 0777)
	if err != nil {
		logger.Errorf("write data to file failed, %s", err)
		return err
	}
	return nil
}

// 读取字符串从文件
func readStringFromFile(filePath string) (string, error) {
	if !isFileExist(filePath) {
		return "", fmt.Errorf("file not exist")
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func getJobFileDir(name string) string {
	return filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME, name)
}

func getJobFilePath(name string) string {
	if name == "" {
		return getJobFileDir("")
	}
	return filepath.Join(getJobFileDir(name), getJobFileName(name))
}

func getJobFileName(name string) string {
	return name + ".yml"
}

func getJobDetailFileDir(name string) string {
	return filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME, name, consts.JOB_DETAIL_DIR_NAME)
}

func getJobDetailFilePath(name string, id int) string {
	return filepath.Join(getJobDetailFileDir(name), getJobDetailFileName(id))
}

func getJobDetailFileName(id int) string {
	return strconv.Itoa(id) + ".yml"
}

func getJobDetailLogPath(name string, id int) string {
	return filepath.Join(getJobDetailLogDir(name), strconv.Itoa(id)+".log")
}

func getJobDetailLogDir(name string) string {
	return filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME, name, consts.JOB_DETAIL_LOG_DIR_NAME)
}

func deleteFile(filePath string) error {
	if !isFileExist(filePath) {
		return fmt.Errorf("file not exist")
	}
	return os.Remove(filePath)
}

func renameFile(oldPath, newPath string) error {
	if oldPath == newPath {
		return nil
	}
	if !isFileExist(oldPath) {
		return fmt.Errorf("file not exist: %s", oldPath)
	}
	if filepath.Dir(oldPath) != filepath.Dir(newPath) {
		err := os.Rename(filepath.Dir(oldPath), filepath.Dir(newPath))
		if err != nil {
			return err
		}
	}
	newFile := filepath.Join(filepath.Dir(newPath), filepath.Base(oldPath))
	return os.Rename(newFile, newPath)
}
