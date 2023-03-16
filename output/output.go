package output

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/utils"
)

type Output struct {
	Name               string
	ID                 int
	buffer             []string
	f                  *os.File
	mu                 sync.Mutex
	filename           string
	fileCursor         int
	bufferCursor       int
	stageTimeConsuming map[string]TimeConsuming
	timeConsuming      TimeConsuming
}

type Log struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Stages    []Stage
	Lines     []string
}

type Stage struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Name      string
	Lines     []string
}

type Step struct {
	Name  string
	Lines []string
}

type TimeConsuming struct {
	Done      bool
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// New 新建一个 Output 对象，会自动初始化文件，以及定时将内容写入文件
func New(name string, id int) *Output {
	o := &Output{
		Name:   name,
		ID:     id,
		buffer: make([]string, 0, 16),
		timeConsuming: TimeConsuming{
			StartTime: time.Now().Local(),
		},
		stageTimeConsuming: make(map[string]TimeConsuming),
		// stepTimeConsuming:  make(map[string]TimeConsuming),
	}

	err := o.initFile()
	if err != nil {
		logger.Errorf("Failed to init output file, err: %s", err)
		return o
	}

	o.timedWriteFile()

	o.WriteLineWithNoTime("[Job] Started on " + o.timeConsuming.StartTime.Format("2006-01-02 15:04:05"))

	return o
}

// Duration 返回持续时间
func (o *Output) Duration() time.Duration {
	if o.timeConsuming.Done {
		return o.timeConsuming.Duration
	}
	return time.Since(o.timeConsuming.StartTime)
}

// TimeConsuming 返回耗时信息
func (o *Output) TimeConsuming() TimeConsuming {
	return o.timeConsuming
}

// StageDuration 返回某个 Stage 的持续时间
func (o *Output) StageDuration(name string) time.Duration {
	stageTimeConsuming, ok := o.stageTimeConsuming[name]
	if !ok {
		return 0
	}
	if stageTimeConsuming.Done {
		return stageTimeConsuming.Duration
	}
	if stageTimeConsuming.StartTime.IsZero() {
		return 0
	}
	return time.Since(stageTimeConsuming.StartTime)
}

// Done 标记输出已完成，会将缓存中的内容刷入文件，然后关闭文件
func (o *Output) Done() {
	logger.Trace("output done, flush all, close file")
	now := time.Now().Local()

	// 将之前的 Stage 标记为完成
	for k, v := range o.stageTimeConsuming {
		if !v.Done {
			v.EndTime = now
			v.Duration = v.EndTime.Sub(v.StartTime)
			v.Done = true
			o.stageTimeConsuming[k] = v
			o.WriteLineWithNoTime(fmt.Sprintf("[TimeConsuming] EndTime: %s, Duration: %s", v.EndTime.Format("2006-01-02 15:04:05"), v.Duration))
		}
	}

	o.mu.Lock()
	o.timeConsuming.Done = true
	o.timeConsuming.EndTime = now
	o.timeConsuming.Duration = now.Sub(o.timeConsuming.StartTime)
	o.flush(o.buffer[o.fileCursor:])
	o.flush([]string{fmt.Sprintf("\n[Job] Finished on %s, Duration: %s\n\n", now.Format("2006-01-02 15:04:05"), o.timeConsuming.Duration)})
	o.f.Close()
	o.mu.Unlock()
}

// WriteLine 将一行普通内容写入输出
func (o *Output) WriteLine(line string) {
	timeFormat := fmt.Sprintf("[%s] ", time.Now().Local().Format("2006-01-02 15:04:05"))
	// 如果不是以换行符结尾，自动添加
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	o.buffer = append(o.buffer, timeFormat+line)
}

func (o *Output) WriteLineWithNoTime(line string) {
	// 如果不是以换行符结尾，自动添加
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	o.buffer = append(o.buffer, line)
}

// WriteCommandLine 将一行命令行内容写入输出，其实就是在前面加上了一个 "> "
func (o *Output) WriteCommandLine(line string) {
	o.WriteLine("> " + line)
}

// Content 总是返回从起始到现在的所有内容
func (o *Output) Content() string {
	o.bufferCursor = len(o.buffer)
	return strings.Join(o.buffer[:o.bufferCursor], "")
}

// NewContent 总是返回自上次读取后新出现的内容
func (o *Output) NewContent() string {
	if o.bufferCursor >= len(o.buffer) {
		return ""
	}
	endIndex := len(o.buffer)
	result := strings.Join(o.buffer[o.bufferCursor:endIndex], "")
	o.bufferCursor = endIndex
	return result
}

// NewStage 会写入以 [Pipeline] Stage: 开头的一行，表示一个新的 Stage 开始
func (o *Output) NewStage(name string) {
	// 将之前的 Stage 标记为完成
	for k, v := range o.stageTimeConsuming {
		if !v.Done {
			v.EndTime = time.Now().Local()
			v.Duration = v.EndTime.Sub(v.StartTime)
			v.Done = true
			o.stageTimeConsuming[k] = v
			o.WriteLineWithNoTime(fmt.Sprintf("[TimeConsuming] EndTime: %s, Duration: %s", v.EndTime.Format("2006-01-02 15:04:05"), v.Duration))
			// o.WriteLineWithNoTime("} ")
		}
	}

	o.WriteLineWithNoTime("\n")
	o.WriteLineWithNoTime("[Pipeline] Stage: " + name)
	// o.WriteLineWithNoTime("{ ")

	startTime := time.Now().Local()
	o.WriteLineWithNoTime("[TimeConsuming] StartTime: " + startTime.Format("2006-01-02 15:04:05"))
	o.stageTimeConsuming[name] = TimeConsuming{
		StartTime: startTime,
	}
}

// NewStep 会写入以 [Pipeline] Step: 开头的一行，表示一个新的 Step 开始
func (o *Output) NewStep(name string) {
	o.WriteLineWithNoTime("[Pipeline] Step: " + name)
}

// 在一个协程中定时刷入文件
func (o *Output) timedWriteFile() {
	endIndex := 0
	go func(endIndex int) {
		for {
			o.mu.Lock()
			if o.timeConsuming.Done {
				o.mu.Unlock()
				break
			}
			o.mu.Unlock()

			if len(o.buffer) <= endIndex {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			endIndex = len(o.buffer)
			err := o.flush(o.buffer[o.fileCursor:endIndex])
			if err != nil {
				logger.Error(err)
			}
			o.fileCursor = endIndex
			time.Sleep(500 * time.Millisecond)
		}
	}(endIndex)
}

// 刷入文件
func (o *Output) flush(arr []string) error {
	if o.f == nil {
		return nil
	}
	for _, line := range arr {
		if _, err := o.f.WriteString(line); err != nil {
			logger.Error(err)
			return err
		}
	}
	return nil
}

// 初始化文件
func (o *Output) initFile() error {
	o.mu.Lock()
	if o.f != nil {
		o.mu.Unlock()
		return nil
	}

	if o.filename == "" {
		o.filename = filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME, o.Name, consts.JOB_DETAIL_LOG_DIR_NAME, fmt.Sprintf("%d.log", o.ID))
	}

	basepath := filepath.Dir(o.filename)
	if err := os.MkdirAll(basepath, 0755); err != nil {
		o.mu.Unlock()
		return err
	}

	f, err := os.OpenFile(o.filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		logger.Errorf("Failed to create output log file %s, err: %s\n", o.filename, err)
		o.mu.Unlock()
		return err
	}
	o.f = f
	o.mu.Unlock()
	logger.Tracef("Create output log file %s success\n", o.filename)
	return nil
}

// Filename 返回文件名
func (o *Output) Filename() string {
	return o.filename
}

// StageOutputList 返回存储了 Stage 输出的列表
func (o *Output) StageOutputList() []Stage {
	return parseLogLines(o.buffer[:]).Stages
}

// ParseLogFile 解析日志文件，返回 Log 对象
func ParseLogFile(filename string) (Log, error) {
	lines, err := ReadFileLines(filename)
	if err != nil {
		return Log{}, err
	}
	result := parseLogLines(lines)
	return result, nil
}

// ReadFileLines 读取文件中的行
func ReadFileLines(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fileScanner := bufio.NewScanner(f)
	fileScanner.Split(bufio.ScanLines)
	var lines []string
	for fileScanner.Scan() {
		lines = append(lines, fileScanner.Text())
	}
	return lines, nil
}

func parseLogLines(lines []string) Log {
	var log Log
	log.Lines = lines

	var stageName = "unknown"
	var stageNameList []string

	// 先遍历到 map 里，由于 map 是无序的，所以需要一个数组来记录顺序
	var stageOutputMap = make(map[string][]string)
	for _, line := range lines {
		if strings.HasPrefix(line, "[Job]") || line == "\n" || line == "" {
			if strings.HasPrefix(line, "[Job] Started on ") {
				startTime := strings.TrimPrefix(line, "[Job] Started on ")
				log.StartTime, _ = time.Parse("2006-01-02 15:04:05", startTime)
			}
			if strings.HasPrefix(line, "[Job] Finished on ") {
				endTimeAndDuration := strings.TrimPrefix(line, "[Job] Finished on ")
				endTimeAndDurationSlice := strings.Split(endTimeAndDuration, ",")
				endTime := endTimeAndDurationSlice[0]
				log.EndTime, _ = time.Parse("2006-01-02 15:04:05", endTime)

				if len(endTimeAndDurationSlice) > 1 {
					duration := endTimeAndDurationSlice[1]
					duration = strings.TrimPrefix(duration, " Duration: ")
					log.Duration, _ = time.ParseDuration(duration)
				}
			}

			continue
		}
		if strings.HasPrefix(line, "[Pipeline] Stage: ") {
			stageName = strings.TrimPrefix(line, "[Pipeline] Stage: ")
			stageOutputMap[stageName] = make([]string, 0)
			stageNameList = append(stageNameList, stageName)
		}
		stageOutputMap[stageName] = append(stageOutputMap[stageName], line)
	}

	for k, v := range stageOutputMap {
		for i := range stageNameList {
			if stageNameList[i] == k {
				var startTime, endTime time.Time
				var duration time.Duration
				for _, line := range v {
					if strings.HasPrefix(line, "[TimeConsuming] StartTime: ") {
						startTimeString := strings.TrimPrefix(line, "[TimeConsuming] StartTime: ")
						startTime, _ = time.Parse("2006-01-02 15:04:05", startTimeString)
					}
					if strings.HasPrefix(line, "[TimeConsuming] EndTime: ") {
						endTimeString := strings.TrimPrefix(line, "[TimeConsuming] EndTime: ")
						endTimeAndDurationSlice := strings.Split(endTimeString, ",")
						endTime, _ = time.Parse("2006-01-02 15:04:05", endTimeAndDurationSlice[0])

						if len(endTimeAndDurationSlice) > 1 {
							durationString := endTimeAndDurationSlice[1]
							durationString = strings.TrimPrefix(durationString, " Duration: ")
							duration, _ = time.ParseDuration(durationString)
						}
					}
				}

				stageOutput := Stage{
					Name:      k,
					Lines:     v,
					StartTime: startTime,
					EndTime:   endTime,
					Duration:  duration,
				}
				log.Stages = append(log.Stages, stageOutput)
			}
		}
	}

	return log
}

// ParseStageSteps 解析一个 stage 中的 step
func ParseStageSteps(stage *Stage) []*Step {
	var stepNameList []string
	var stepMap = make(map[string]*Step)
	for _, s := range stage.Lines {
		if strings.HasPrefix("[TimeConsuming]", s) {
			continue
		}
		// 如果以 [Pipeline] Step: 开头，那么就是一个 step
		if strings.HasPrefix(s, "[Pipeline] Step: ") {
			stepName := strings.TrimPrefix(s, "[Pipeline] Step: ")
			stepNameList = append(stepNameList, stepName)
			stepMap[stepName] = &Step{Name: stepName}
			continue
		}
		if len(stepNameList) == 0 {
			continue
		}
		stepName := stepNameList[len(stepNameList)-1]
		step := stepMap[stepName]
		step.Lines = append(step.Lines, s)
	}
	var result []*Step
	for i := range stepNameList {
		for k, v := range stepMap {
			if k == stepNameList[i] {
				result = append(result, v)
			}
		}
	}
	return result
}
