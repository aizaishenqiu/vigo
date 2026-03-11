package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpression Cron 表达式解析结果
type CronExpression struct {
	Second     []int
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int
}

// ParseCronExpression 解析 Cron 表达式
// 支持 5 位和 6 位表达式
// 5 位：分 时 日 月 周
// 6 位：秒 分 时 日 月 周
func ParseCronExpression(expr string) (*CronExpression, error) {
	if expr == "" {
		return nil, errors.New("cron expression cannot be empty")
	}

	// 去除多余空格并分割
	fields := strings.Fields(expr)
	if len(fields) != 5 && len(fields) != 6 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 or 6 fields, got %d", len(fields))
	}

	var err error
	result := &CronExpression{}

	// 根据 5 位或 6 位表达式解析
	if len(fields) == 6 {
		// 6 位：秒 分 时 日 月 周
		result.Second, err = parseField(fields[0], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("invalid second field: %w", err)
		}
		result.Minute, err = parseField(fields[1], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("invalid minute field: %w", err)
		}
		result.Hour, err = parseField(fields[2], 0, 23)
		if err != nil {
			return nil, fmt.Errorf("invalid hour field: %w", err)
		}
		result.DayOfMonth, err = parseDayOfMonthField(fields[3])
		if err != nil {
			return nil, fmt.Errorf("invalid day of month field: %w", err)
		}
		result.Month, err = parseField(fields[4], 1, 12)
		if err != nil {
			return nil, fmt.Errorf("invalid month field: %w", err)
		}
		result.DayOfWeek, err = parseDayOfWeekField(fields[5])
		if err != nil {
			return nil, fmt.Errorf("invalid day of week field: %w", err)
		}
	} else {
		// 5 位：分 时 日 月 周
		result.Minute, err = parseField(fields[0], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("invalid minute field: %w", err)
		}
		result.Hour, err = parseField(fields[1], 0, 23)
		if err != nil {
			return nil, fmt.Errorf("invalid hour field: %w", err)
		}
		result.DayOfMonth, err = parseDayOfMonthField(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid day of month field: %w", err)
		}
		result.Month, err = parseField(fields[3], 1, 12)
		if err != nil {
			return nil, fmt.Errorf("invalid month field: %w", err)
		}
		result.DayOfWeek, err = parseDayOfWeekField(fields[4])
		if err != nil {
			return nil, fmt.Errorf("invalid day of week field: %w", err)
		}
		// 秒字段默认为 0
		result.Second = []int{0}
	}

	return result, nil
}

// parseField 解析单个字段
// 支持: *, 数字, 范围(1-5), 步长(*/5), 列表(1,3,5), 组合(1-5/2)
func parseField(field string, min, max int) ([]int, error) {
	field = strings.TrimSpace(field)

	// 处理 * (所有值)
	if field == "*" {
		return generateRange(min, max), nil
	}

	// 处理 ? (任何值，通常用于日和周)
	if field == "?" {
		return []int{}, nil // 空数组表示"无限制"
	}

	// 处理步长 (*/5, 1-10/2)
	if strings.Contains(field, "/") {
		return parseStepField(field, min, max)
	}

	// 处理范围 (1-5)
	if strings.Contains(field, "-") {
		return parseRangeField(field, min, max)
	}

	// 处理列表 (1,3,5)
	if strings.Contains(field, ",") {
		return parseListField(field, min, max)
	}

	// 处理单个数字
	value, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("invalid value: %s", field)
	}
	if value < min || value > max {
		return nil, fmt.Errorf("value %d out of range [%d-%d]", value, min, max)
	}

	return []int{value}, nil
}

// parseStepField 解析步长字段 (如 */5 或 1-10/2)
func parseStepField(field string, min, max int) ([]int, error) {
	parts := strings.Split(field, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid step format: %s", field)
	}

	step, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid step value: %s", parts[1])
	}
	if step <= 0 {
		return nil, errors.New("step must be positive")
	}

	var baseValues []int
	if parts[0] == "*" {
		baseValues = generateRange(min, max)
	} else {
		var err error
		baseValues, err = parseField(parts[0], min, max)
		if err != nil {
			return nil, err
		}
	}

	// 应用步长
	result := make([]int, 0)
	for i := 0; i < len(baseValues); i += step {
		result = append(result, baseValues[i])
	}

	return result, nil
}

// parseRangeField 解析范围字段 (如 1-5)
func parseRangeField(field string, min, max int) ([]int, error) {
	parts := strings.Split(field, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format: %s", field)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid start value: %s", parts[0])
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid end value: %s", parts[1])
	}

	if start < min || start > max {
		return nil, fmt.Errorf("start value %d out of range [%d-%d]", start, min, max)
	}
	if end < min || end > max {
		return nil, fmt.Errorf("end value %d out of range [%d-%d]", end, min, max)
	}
	if start > end {
		return nil, fmt.Errorf("start value %d cannot be greater than end value %d", start, end)
	}

	return generateRange(start, end), nil
}

// parseListField 解析列表字段 (如 1,3,5)
func parseListField(field string, min, max int) ([]int, error) {
	parts := strings.Split(field, ",")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid list value: %s", part)
		}
		if value < min || value > max {
			return nil, fmt.Errorf("list value %d out of range [%d-%d]", value, min, max)
		}

		// 避免重复
		duplicate := false
		for _, v := range result {
			if v == value {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, value)
		}
	}

	return result, nil
}

// parseDayOfMonthField 解析日字段（支持 L, W 等特殊字符）
func parseDayOfMonthField(field string) ([]int, error) {
	// 简化实现：暂不支持 L, W
	return parseField(field, 1, 31)
}

// parseDayOfWeekField 解析周字段（支持 L, # 等特殊字符）
func parseDayOfWeekField(field string) ([]int, error) {
	// 简化实现：暂不支持 L, #
	// 支持名称：SUN, MON, TUE, WED, THU, FRI, SAT
	field = strings.ToUpper(field)

	dayNames := map[string]int{
		"SUN": 0, "MON": 1, "TUE": 2, "WED": 3,
		"THU": 4, "FRI": 5, "SAT": 6,
	}

	if dayName, ok := dayNames[field]; ok {
		return []int{dayName}, nil
	}

	return parseField(field, 0, 6)
}

// generateRange 生成从 min 到 max 的数字序列
func generateRange(min, max int) []int {
	result := make([]int, max-min+1)
	for i := 0; i <= max-min; i++ {
		result[i] = min + i
	}
	return result
}

// GetNextRun 获取下次执行时间
func (c *CronExpression) GetNextRun(from time.Time) time.Time {
	// 从 from 的下一分钟开始检查
	next := from.Add(time.Minute).Truncate(time.Minute)
	
	// 最多尝试 4 年（避免无限循环）
	maxIterations := 4 * 365 * 24 * 60
	iterations := 0

	for iterations < maxIterations {
		// 检查是否匹配
		if c.matches(next) {
			return next
		}
		
		// 每次增加一分钟
		next = next.Add(time.Minute)
		iterations++
	}

	// 如果找不到合适时间，返回一年后的时间
	return from.AddDate(1, 0, 0)
}

// matches 检查给定时间是否匹配 Cron 表达式
func (c *CronExpression) matches(t time.Time) bool {
	// 检查秒
	if !containsInt(c.Second, t.Second()) {
		return false
	}
	
	// 检查分
	if !containsInt(c.Minute, t.Minute()) {
		return false
	}
	
	// 检查时
	if !containsInt(c.Hour, t.Hour()) {
		return false
	}
	
	// 检查日
	if len(c.DayOfMonth) > 0 && !containsInt(c.DayOfMonth, t.Day()) {
		return false
	}
	
	// 检查月
	if !containsInt(c.Month, int(t.Month())) {
		return false
	}
	
	// 检查周
	if len(c.DayOfWeek) > 0 && !containsInt(c.DayOfWeek, int(t.Weekday())) {
		return false
	}
	
	return true
}

// containsInt 检查切片是否包含某个值
func containsInt(slice []int, value int) bool {
	if len(slice) == 0 {
		return true // 空数组表示无限制
	}
	
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// ValidateCronExpression 验证 Cron 表达式
func ValidateCronExpression(expr string) error {
	_, err := ParseCronExpression(expr)
	return err
}

// FormatCronExpression 格式化 Cron 表达式
func FormatCronExpression(expr string) (string, error) {
	parsed, err := ParseCronExpression(expr)
	if err != nil {
		return "", err
	}

	// 重建表达式（规范化格式）
	result := ""
	if len(parsed.Second) > 0 {
		result = "0 " // 秒字段默认为 0
	}
	
	result += formatField(parsed.Minute) + " "
	result += formatField(parsed.Hour) + " "
	result += formatField(parsed.DayOfMonth) + " "
	result += formatField(parsed.Month) + " "
	result += formatField(parsed.DayOfWeek)

	return strings.TrimSpace(result), nil
}

// formatField 格式化字段
func formatField(values []int) string {
	if len(values) == 0 {
		return "?"
	}
	
	// 检查是否为连续范围
	if isConsecutive(values) {
		return fmt.Sprintf("%d-%d", values[0], values[len(values)-1])
	}
	
	// 转换为字符串
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = strconv.Itoa(v)
	}
	
	return strings.Join(strValues, ",")
}

// isConsecutive 检查数组是否连续
func isConsecutive(values []int) bool {
	if len(values) < 2 {
		return false
	}
	
	for i := 1; i < len(values); i++ {
		if values[i] != values[i-1]+1 {
			return false
		}
	}
	return true
}

// DescribeCronExpression 描述 Cron 表达式
func DescribeCronExpression(expr string) (string, error) {
	_, err := ParseCronExpression(expr)
	if err != nil {
		return "", err
	}

	// 简化实现：返回基础描述
	parts := strings.Fields(expr)
	
	var desc strings.Builder
	
	if len(parts) == 6 {
		desc.WriteString(fmt.Sprintf("Run at second %s", parts[0]))
	} else {
		desc.WriteString("Run at minute 0")
	}
	
	desc.WriteString(fmt.Sprintf(", hour %s", parts[len(parts)-5]))
	desc.WriteString(fmt.Sprintf(", day of month %s", parts[len(parts)-4]))
	desc.WriteString(fmt.Sprintf(", month %s", parts[len(parts)-3]))
	desc.WriteString(fmt.Sprintf(", day of week %s", parts[len(parts)-2]))
	
	return desc.String(), nil
}
