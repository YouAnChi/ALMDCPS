package gongju

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-ego/gse"
	"github.com/xuri/excelize/v2"
	"io"
)

// ModelParams 模型参数结构
type ModelParams struct {
	ResponseTime  float64 `json:"responseTime"`  // 响应时间（毫秒）
	Accuracy     float64 `json:"accuracy"`      // 准确率（百分比）
	ResourceUsage float64 `json:"resourceUsage"` // 资源消耗（MB）
}

// ModelScoreResponse 模型分值计算响应结构
type ModelScoreResponse struct {
	Score    float64 `json:"score"`
	Analysis string  `json:"analysis"`
}

// WordType 词语类型
type WordType int

const (
	TypeOther WordType = iota // 其他词
	TypeVerb                  // 动词
	TypeNoun                  // 名词
	TypeAdj                   // 形容词
	TypeAdv                   // 副词
)

// WordMatch 存储词语匹配信息
type WordMatch struct {
	word     string
	score    float64
	position int
	wordType WordType
}

// matchResult 匹配结果
type matchResult struct {
	exactMatches       float64
	semanticMatches    float64
	positionAwareScore float64
}

// CiLinCode 哈工大词林编码结构
type CiLinCode struct {
	FirstLevel  string
	SecondLevel string
	ThirdLevel  string
	FourthLevel string
	FifthLevel  string
}

// SynonymDict 同义词字典
type SynonymDict struct {
	cilinMap map[string]CiLinCode
	codemap  map[string][]string
	mu       sync.RWMutex
}

// TextSimilarity 存储文本相似度的各种指标
type TextSimilarity struct {
	F1              float64
	Precision       float64
	Recall          float64
	SemanticF1      float64
	PositionAwareF1 float64
}

// ProcessResponse 处理响应结构
type ProcessResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	ResultFile string `json:"resultFile,omitempty"`
}

// 全局变量
var (
	globalSynonymDict = &SynonymDict{
		cilinMap: make(map[string]CiLinCode),
		codemap:  make(map[string][]string),
	}
	segmenter gse.Segmenter
	jiebaLock    sync.Mutex
	initialized  bool
	initLock     sync.Once
)

// 初始化函数
func initialize() error {
	// 初始化分词器
	err := segmenter.LoadDict()
	if err != nil {
		log.Printf("警告：初始化分词器失败: %v，将使用默认分词", err)
	}

	// 加载同义词词典
	dictPath := "gongju/cilin.txt"
	err = globalSynonymDict.LoadCiLinDict(dictPath)
	if err != nil {
		log.Printf("警告：加载同义词词典失败: %v，将不使用同义词功能", err)
	}

	initialized = true
	return nil
}

// CalculateModelScore 计算智能大模型分值
func CalculateModelScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持POST请求", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求参数
	var params ModelParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "参数解析失败", http.StatusBadRequest)
		return
	}

	// 计算综合得分
	// 1. 响应时间得分（越低越好，满分40分）
	responseTimeScore := 40.0 * (1.0 - (params.ResponseTime / 1000.0))
	if responseTimeScore < 0 {
		responseTimeScore = 0
	}

	// 2. 准确率得分（直接转换为40分制）
	accuracyScore := 40.0 * (params.Accuracy / 100.0)

	// 3. 资源消耗得分（越低越好，满分20分）
	resourceScore := 20.0 * (1.0 - (params.ResourceUsage / 1000.0))
	if resourceScore < 0 {
		resourceScore = 0
	}

	// 计算总分
	totalScore := responseTimeScore + accuracyScore + resourceScore

	// 生成分析报告
	analysis := generateAnalysis(params, responseTimeScore, accuracyScore, resourceScore)

	response := ModelScoreResponse{
		Score:    totalScore,
		Analysis: analysis,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateAnalysis 生成分析报告
func generateAnalysis(params ModelParams, responseTimeScore, accuracyScore, resourceScore float64) string {
	var analysis string

	// 响应时间分析
	if responseTimeScore >= 30 {
		analysis += "响应时间表现优秀，"
	} else if responseTimeScore >= 20 {
		analysis += "响应时间表现良好，"
	} else {
		analysis += "响应时间需要优化，"
	}

	// 准确率分析
	if accuracyScore >= 35 {
		analysis += "准确率非常高，"
	} else if accuracyScore >= 25 {
		analysis += "准确率表现良好，"
	} else {
		analysis += "准确率有待提高，"
	}

	// 资源消耗分析
	if resourceScore >= 15 {
		analysis += "资源利用效率高。"
	} else if resourceScore >= 10 {
		analysis += "资源消耗适中。"
	} else {
		analysis += "资源消耗较大，建议优化。"
	}

	return analysis
}

// ProcessExcelFile 处理Excel文件并计算语义相似度
func ProcessExcelFile(w http.ResponseWriter, r *http.Request) {
	// 确保已初始化
	initLock.Do(func() {
		if err := initialize(); err != nil {
			log.Printf("初始化失败: %v", err)
		}
	})

	if !initialized {
		http.Error(w, "系统未正确初始化", http.StatusInternalServerError)
		return
	}

	// 检查请求方法
	if r.Method != http.MethodPost {
		http.Error(w, "只支持POST请求", http.StatusMethodNotAllowed)
		return
	}

	// 解析文件
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "获取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 检查文件类型
	if !strings.HasSuffix(header.Filename, ".xlsx") {
		http.Error(w, "只支持.xlsx格式的文件", http.StatusBadRequest)
		return
	}

	// 创建临时文件
	tempFile := excelize.NewFile()
	defer tempFile.Close()

	// 读取上传的文件
	uploadedFile, err := excelize.OpenReader(file)
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusInternalServerError)
		return
	}
	defer uploadedFile.Close()

	// 获取第一个工作表
	sheetName := uploadedFile.GetSheetName(0)
	rows, err := uploadedFile.GetRows(sheetName)
	if err != nil {
		http.Error(w, "读取工作表失败", http.StatusInternalServerError)
		return
	}

	// 设置表头
	headers := []string{"标准答案", "预测文本", "语义F1值"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		tempFile.SetCellValue("Sheet1", cell, header)
	}

	// 处理每一行
	for rowIdx, row := range rows {
		if rowIdx == 0 || len(row) < 2 {
			continue
		}

		actual := strings.TrimSpace(row[0])
		predicted := strings.TrimSpace(row[1])

		// 计算相似度
		similarity := calculateSemanticF1(actual, predicted, segmenter)

		// 写入结果
		rowNum := rowIdx + 1
		tempFile.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowNum), actual)
		tempFile.SetCellValue("Sheet1", fmt.Sprintf("B%d", rowNum), predicted)
		tempFile.SetCellValue("Sheet1", fmt.Sprintf("C%d", rowNum), similarity.SemanticF1)
	}

	// 调整列宽
	tempFile.SetColWidth("Sheet1", "A", "C", 30)

	// 生成结果文件名
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	resultFileName := fmt.Sprintf("语义F1值_%s.xlsx", timestamp)
	resultPath := filepath.Join("uploads", resultFileName)

	// 保存结果文件
	if err := tempFile.SaveAs(resultPath); err != nil {
		http.Error(w, "保存结果文件失败", http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := ProcessResponse{
		Status:     "success",
		Message:    "文件处理成功",
		ResultFile: "/uploads/" + resultFileName,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CalculateACCScore 计算ACC分数
func CalculateACCScore(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 解析文件
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "获取文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// 检查文件扩展名
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "不支持的文件类型，请上传.xlsx文件",
		})
		return
	}

	// 创建临时文件
	tempDir := "./uploads"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "创建临时目录失败: " + err.Error(),
		})
		return
	}

	tempFile := filepath.Join(tempDir, fmt.Sprintf("acc_result_%d.xlsx", time.Now().Unix()))
	dst, err := os.Create(tempFile)
	if err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "创建临时文件失败: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	// 保存上传的文件
	if _, err := io.Copy(dst, file); err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "保存文件失败: " + err.Error(),
		})
		return
	}

	// 打开Excel文件
	f, err := excelize.OpenFile(tempFile)
	if err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "打开Excel文件失败: " + err.Error(),
		})
		return
	}
	defer f.Close()

	// 获取第一个工作表名称
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "Excel文件中没有工作表",
		})
		return
	}
	sheet := sheets[0]

	// 获取所有行
	rows, err := f.GetRows(sheet)
	if err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "读取工作表失败: " + err.Error(),
		})
		return
	}

	// 处理每一行数据
	for i, row := range rows {
		if len(row) < 2 {
			continue // 跳过不完整的行
		}

		// 对两列文本进行分词
		words1 := splitWords(row[0])
		words2 := splitWords(row[1])

		// 计算ACC值
		acc := calculateACC(words1, words2)

		// 将结果写入C列
		cell := fmt.Sprintf("C%d", i+1)
		if err := f.SetCellValue(sheet, cell, acc); err != nil {
			log.Printf("写入结果失败: %v\n", err)
			continue
		}
	}

	// 保存结果文件
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	resultFile := filepath.Join(tempDir, fmt.Sprintf("ACC结果_%s.xlsx", timestamp))
	if err := f.SaveAs(resultFile); err != nil {
		json.NewEncoder(w).Encode(ProcessResponse{
			Status:  "error",
			Message: "保存结果文件失败: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	json.NewEncoder(w).Encode(ProcessResponse{
		Status:     "success",
		Message:    "ACC分数计算完成",
		ResultFile: "/uploads/" + filepath.Base(resultFile),
	})
}

// splitWords 将文本分词并返回词列表
func splitWords(text string) []string {
	if text == "" {
		return []string{}
	}
	// 使用互斥锁保护分词操作
	return segmenter.CutAll(text)
}

// calculateACC 计算两个词列表的ACC值
func calculateACC(words1, words2 []string) float64 {
	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	// 完全匹配才返回1，否则返回0
	if words1[0] == words2[0] {
		return 1
	}
	return 0
}

// ParseCiLinCode 解析词林编码
func ParseCiLinCode(code string) CiLinCode {
	return CiLinCode{
		FirstLevel:  code[0:1],
		SecondLevel: code[1:2],
		ThirdLevel:  code[2:4],
		FourthLevel: code[4:5],
		FifthLevel:  code[5:7],
	}
}

// LoadCiLinDict 加载词林词典
func (sd *SynonymDict) LoadCiLinDict(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			continue
		}

		code := parts[0]
		cilinCode := ParseCiLinCode(code)
		words := parts[1:]

		sd.mu.Lock()
		sd.codemap[code] = words
		for _, word := range words {
			sd.cilinMap[word] = cilinCode
		}
		sd.mu.Unlock()
	}

	return scanner.Err()
}

// GetSynonyms 获取同义词
func (sd *SynonymDict) GetSynonyms(word string) []string {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if code, ok := sd.cilinMap[word]; ok {
		codeStr := fmt.Sprintf("%s%s%s%s%s",
			code.FirstLevel,
			code.SecondLevel,
			code.ThirdLevel,
			code.FourthLevel,
			code.FifthLevel)
		if words, exists := sd.codemap[codeStr]; exists {
			synonyms := make([]string, 0)
			for _, w := range words {
				if w != word {
					synonyms = append(synonyms, w)
				}
			}
			return synonyms
		}
	}
	return nil
}

// 获取词语类型
func getWordType(word string) WordType {
	code, ok := globalSynonymDict.cilinMap[word]
	if !ok {
		return TypeOther
	}

	// 根据词林编码判断词语类型
	switch code.FirstLevel {
	case "A", "B", "C": // 名词相关
		return TypeNoun
	case "D", "E", "F": // 动词相关
		return TypeVerb
	case "G", "H": // 形容词相关
		return TypeAdj
	case "K": // 副词
		return TypeAdv
	default:
		return TypeOther
	}
}

// 获取词语类型的位置容忍度
func getPositionTolerance(wordType WordType) float64 {
	switch wordType {
	case TypeVerb:
		return 0.3 // 动词位置相对固定
	case TypeNoun:
		return 0.4 // 名词位置较为灵活
	case TypeAdj:
		return 0.5 // 形容词位置更灵活
	case TypeAdv:
		return 0.6 // 副词位置最灵活
	default:
		return 0.4 // 默认容忍度
	}
}

// calculateMatches 计算词语匹配情况
func calculateMatches(actual, predicted []WordMatch, actualLen, predictedLen int) matchResult {
	exactMatches := 0.0
	semanticMatches := 0.0
	positionAwareScore := 0.0

	// 创建访问标记
	usedPredicted := make([]bool, len(predicted))

	// 首先处理完全匹配
	for _, actualWord := range actual {
		actualType := getWordType(actualWord.word)
		tolerance := getPositionTolerance(actualType)

		bestMatchScore := 0.0
		bestPositionScore := 0.0
		bestMatchIdx := -1

		// 找到最佳匹配
		for j, predictedWord := range predicted {
			if usedPredicted[j] {
				continue
			}

			matchScore := getMatchScore(actualWord.word, predictedWord.word)
			if matchScore > 0 {
				predictedType := getWordType(predictedWord.word)

				// 计算位置分数
				positionScore := calculatePositionScore(
					float64(actualWord.position)/float64(actualLen),
					float64(predictedWord.position)/float64(predictedLen),
					tolerance,
				)

				// 根据词语类型调整分数，但确保不超过1.0
				if actualType == predictedType && matchScore < 1.0 {
					// 只增加剩余空间的10%
					matchScore = matchScore + (1.0 - matchScore) * 0.1
				}

				// 更新最佳匹配
				totalScore := matchScore * positionScore
				if totalScore > bestMatchScore {
					bestMatchScore = matchScore
					bestPositionScore = positionScore
					bestMatchIdx = j
				}
			}
		}

		// 使用最佳匹配更新分数
		if bestMatchIdx >= 0 {
			if bestMatchScore == 1.0 {
				exactMatches++
			}
			semanticMatches += bestMatchScore
			positionAwareScore += bestMatchScore * bestPositionScore
			usedPredicted[bestMatchIdx] = true
		}
	}

	return matchResult{
		exactMatches:       exactMatches,
		semanticMatches:    semanticMatches,
		positionAwareScore: positionAwareScore,
	}
}

// calculatePositionScore 计算位置相似度分数
func calculatePositionScore(pos1, pos2 float64, tolerance float64) float64 {
	diff := math.Abs(pos1 - pos2)

	// 如果在容忍范围内，给予较高分数
	if diff <= tolerance {
		return 1.0 - (diff/tolerance)*0.2 // 在容忍范围内最多扣除20%的分数
	}

	// 超出容忍范围，使用更温和的衰减率
	return math.Exp(-1.5 * (diff - tolerance) * (diff - tolerance))
}

// getMatchScore 获取两个词的匹配分数
func getMatchScore(word1, word2 string) float64 {
	// 完全匹配
	if word1 == word2 {
		return 1.0
	}

	// 检查同义词
	if synonyms := globalSynonymDict.GetSynonyms(word1); synonyms != nil {
		for _, syn := range synonyms {
			if syn == word2 {
				return 0.9
			}
		}
	}

	// 字符重叠度计算
	if len(word1) >= 2 && len(word2) >= 2 {
		overlap := calculateCharacterOverlap(word1, word2)
		if overlap > 0.5 {
			return overlap * 0.8
		}
	}

	return 0
}

// calculateCharacterOverlap 计算字符重叠度
func calculateCharacterOverlap(word1, word2 string) float64 {
	chars1 := strings.Split(word1, "")
	chars2 := strings.Split(word2, "")

	common := 0
	for _, c1 := range chars1 {
		for _, c2 := range chars2 {
			if c1 == c2 {
				common++
				break
			}
		}
	}

	return float64(common) / math.Max(float64(len(chars1)), float64(len(chars2)))
}

// calculateF1Score 计算F1分数
func calculateF1Score(precision, recall float64) float64 {
	if precision+recall == 0 {
		return 0
	}
	// 确保precision和recall不超过1.0
	precision = math.Min(1.0, precision)
	recall = math.Min(1.0, recall)
	return 2 * (precision * recall) / (precision + recall)
}

// safeDiv 安全除法
func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// calculateSemanticF1 计算语义相似度
func calculateSemanticF1(actual, predicted string, seg gse.Segmenter) TextSimilarity {
	// 分词
	actualWords := seg.Cut(actual, true)
	predictedWords := seg.Cut(predicted, true)

	// 创建词频和位置映射
	actualMatches := make([]WordMatch, 0)
	predictedMatches := make([]WordMatch, 0)

	// 记录词语位置
	for i, word := range actualWords {
		actualMatches = append(actualMatches, WordMatch{
			word:     word,
			position: i,
			score:    1.0,
			wordType: getWordType(word),
		})
	}

	for i, word := range predictedWords {
		predictedMatches = append(predictedMatches, WordMatch{
			word:     word,
			position: i,
			score:    1.0,
			wordType: getWordType(word),
		})
	}

	// 计算匹配分数
	matches := calculateMatches(actualMatches, predictedMatches, len(actualWords), len(predictedWords))

	// 计算基础指标
	truePositives := matches.exactMatches
	semanticPositives := matches.semanticMatches
	totalActual := float64(len(actualWords))
	totalPredicted := float64(len(predictedWords))

	// 计算基础F1
	precision := safeDiv(float64(truePositives), totalPredicted)
	recall := safeDiv(float64(truePositives), totalActual)
	basicF1 := calculateF1Score(precision, recall)

	// 计算语义F1
	semanticPrecision := safeDiv(semanticPositives, totalPredicted)
	semanticRecall := safeDiv(semanticPositives, totalActual)
	semanticF1 := calculateF1Score(semanticPrecision, semanticRecall)

	// 计算位置感知F1
	positionPrecision := safeDiv(matches.positionAwareScore, totalPredicted)
	positionRecall := safeDiv(matches.positionAwareScore, totalActual)
	positionF1 := calculateF1Score(positionPrecision, positionRecall)

	return TextSimilarity{
		F1:              basicF1,
		Precision:       precision,
		Recall:          recall,
		SemanticF1:      semanticF1,
		PositionAwareF1: positionF1,
	}
}
