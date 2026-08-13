// Package ocr 实现统一认证验证码的本地 OCR。
//
// 算法与权重移植自浏览器扩展 scu-plus（GPL-3.0）：
// 质心模板匹配，特征为 48 维 8×6 二值像素 + 1 维宽高比 = 49 维，
// 36 类（0-9a-z）uint8 质心，欧氏距离 + 宽高比加权。
// 预处理：白底合成 → 去灰线（精确颜色匹配）→ 颜色量化分割 4 字符 →
// 逐字符裁剪/缩放/二值化 → 质心匹配。
package ocr

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
	"strings"
	"sync"
)

const (
	captchaWidth  = 80
	captchaHeight = 26
	charH         = 8
	charW         = 6
	featDim       = charH * charW // 48
	featDimWithAR = featDim + 1   // 49
	numClasses    = 36
	captchaLen    = 4

	// 预处理常量（与训练一致）
	lineTolerance  = 10
	colorQuantStep = 8
	whiteThreshold = 250
	binarizeThresh = 0.3
	minConfidence  = 0.3
	maxAspectRatio = 2.0
	arWeight       = 25.0
)

const charset = "0123456789abcdefghijklmnopqrstuvwxyz"

// lineColorRGB 灰线颜色 RGB（来自验证码生成代码 fill="#6f6e70"）。
var lineColorRGB = [3]int{111, 110, 112}

//go:embed model_centroid.json
var modelJSON []byte

type centroidModel struct {
	CentroidsB64 string `json:"centroids_b64"`
}

var (
	centroidsOnce sync.Once
	centroids     []float32
	centroidsErr  error
)

// loadCentroids 懒加载并缓存质心模板（uint8 → [0,1] float32）。
func loadCentroids() ([]float32, error) {
	centroidsOnce.Do(func() {
		var m centroidModel
		if err := json.Unmarshal(modelJSON, &m); err != nil {
			centroidsErr = fmt.Errorf("OCR 模型解析失败: %w", err)
			return
		}
		raw, err := base64.StdEncoding.DecodeString(m.CentroidsB64)
		if err != nil {
			centroidsErr = fmt.Errorf("OCR 模型 base64 解码失败: %w", err)
			return
		}
		if len(raw) != numClasses*featDimWithAR {
			centroidsErr = fmt.Errorf("OCR 模型维度异常: got %d, want %d", len(raw), numClasses*featDimWithAR)
			return
		}
		centroids = make([]float32, len(raw))
		for i, b := range raw {
			centroids[i] = float32(b) / 255.0
		}
	})
	return centroids, centroidsErr
}

// RecognizeBase64 识别 base64 编码的验证码图片（可带 data URL 前缀）。
// 返回 4 位字符；置信度过低或分割失败返回 ("", nil)。
func RecognizeBase64(b64 string) (string, error) {
	// 去 data URL 前缀（data:image/...;base64,）
	if i := strings.Index(b64, ","); i >= 0 && i < 64 {
		b64 = b64[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("验证码图片 base64 解码失败: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("验证码图片解码失败: %w", err)
	}
	return Recognize(img), nil
}

// Recognize 识别验证码图片。返回 4 位字符；失败返回空串。
func Recognize(img image.Image) string {
	rgb := toRGB(img)
	cleaned := removeGrayLines(rgb)
	chars := segmentCharacters(cleaned)
	if len(chars) != captchaLen {
		return ""
	}
	cents, err := loadCentroids()
	if err != nil {
		return ""
	}
	text := make([]byte, 0, captchaLen)
	minConf := 1.0
	for _, ch := range chars {
		feature := make([]float32, featDimWithAR)
		copy(feature, ch.image[:])
		bw := float64(ch.bbox[2] - ch.bbox[0] + 1)
		bh := float64(ch.bbox[3] - ch.bbox[1] + 1)
		ar := bw / math.Max(bh, 1)
		feature[featDim] = float32(normalizeAspectRatio(ar))
		idx, conf := classifyChar(feature, cents)
		text = append(text, charset[idx])
		if conf < minConf {
			minConf = conf
		}
	}
	if minConf < minConfidence {
		return ""
	}
	return string(text)
}

// toRGB 把图片转换为 80×26 RGB（白底合成；过大裁左上角，过小最近邻放大）。
func toRGB(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rgb := make([]uint8, captchaWidth*captchaHeight*3)
	for y := 0; y < captchaHeight; y++ {
		for x := 0; x < captchaWidth; x++ {
			var sx, sy int
			if w >= captchaWidth && h >= captchaHeight {
				sx, sy = b.Min.X+x, b.Min.Y+y
			} else {
				sx = b.Min.X + x*w/captchaWidth
				sy = b.Min.Y + y*h/captchaHeight
			}
			r, g, bl, a := img.At(sx, sy).RGBA()
			// 白底合成（16bit → 8bit）
			alpha := float64(a) / 65535.0
			p := (y*captchaWidth + x) * 3
			rgb[p] = uint8((float64(r>>8)*alpha + 255*(1-alpha)) + 0.5)
			rgb[p+1] = uint8((float64(g>>8)*alpha + 255*(1-alpha)) + 0.5)
			rgb[p+2] = uint8((float64(bl>>8)*alpha + 255*(1-alpha)) + 0.5)
		}
	}
	return rgb
}

func isLineColor(r, g, b int) bool {
	return abs(r-lineColorRGB[0]) <= lineTolerance &&
		abs(g-lineColorRGB[1]) <= lineTolerance &&
		abs(b-lineColorRGB[2]) <= lineTolerance
}

// removeGrayLines 精确颜色匹配去灰线（填白）。
func removeGrayLines(rgb []uint8) []uint8 {
	cleaned := make([]uint8, len(rgb))
	for p := 0; p < len(rgb); p += 3 {
		r, g, b := int(rgb[p]), int(rgb[p+1]), int(rgb[p+2])
		if isLineColor(r, g, b) {
			cleaned[p], cleaned[p+1], cleaned[p+2] = 255, 255, 255
		} else {
			cleaned[p], cleaned[p+1], cleaned[p+2] = rgb[p], rgb[p+1], rgb[p+2]
		}
	}
	return cleaned
}

type charImage struct {
	image [featDim]float32
	bbox  [4]int // x1, y1, x2, y2
}

type colorCenter struct {
	r, g, b int
}

// segmentCharacters 按颜色分割字符，返回按 x 中心排序的字符。
func segmentCharacters(rgb []uint8) []charImage {
	const w, h = captchaWidth, captchaHeight
	length := w * h

	type pixel struct {
		r, g, b, idx int
	}
	var charPixels []pixel
	for i := 0; i < length; i++ {
		p := i * 3
		r, g, b := int(rgb[p]), int(rgb[p+1]), int(rgb[p+2])
		if !(r > whiteThreshold && g > whiteThreshold && b > whiteThreshold) {
			charPixels = append(charPixels, pixel{r, g, b, i})
		}
	}
	if len(charPixels) < 20 {
		return nil
	}

	// 颜色量化，取最频繁的 4 种颜色中心
	type accum struct {
		rSum, gSum, bSum, count int
	}
	quant := map[[3]int]*accum{}
	for _, px := range charPixels {
		key := [3]int{px.r / colorQuantStep * colorQuantStep, px.g / colorQuantStep * colorQuantStep, px.b / colorQuantStep * colorQuantStep}
		a, ok := quant[key]
		if !ok {
			a = &accum{}
			quant[key] = a
		}
		a.rSum += px.r
		a.gSum += px.g
		a.bSum += px.b
		a.count++
	}
	if len(quant) < 4 {
		return nil
	}
	type qa struct {
		key [3]int
		a   *accum
	}
	var sorted []qa
	for k, v := range quant {
		sorted = append(sorted, qa{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].a.count > sorted[j].a.count })
	var centers []colorCenter
	for _, item := range sorted[:4] {
		centers = append(centers, colorCenter{
			r: item.a.rSum / item.a.count,
			g: item.a.gSum / item.a.count,
			b: item.a.bSum / item.a.count,
		})
	}

	// 每个像素归属到最近颜色中心
	labels := make([]int, length)
	for i := range labels {
		labels[i] = -1
	}
	for _, px := range charPixels {
		bestDist := math.MaxInt32
		bestC := -1
		for c, ctr := range centers {
			d := (px.r-ctr.r)*(px.r-ctr.r) + (px.g-ctr.g)*(px.g-ctr.g) + (px.b-ctr.b)*(px.b-ctr.b)
			if d < bestDist {
				bestDist = d
				bestC = c
			}
		}
		labels[px.idx] = bestC
	}

	var chars []charImage
	for c := range centers {
		x1, y1, x2, y2 := w, h, -1, -1
		count := 0
		for i := 0; i < length; i++ {
			if labels[i] != c {
				continue
			}
			count++
			x, y := i%w, i/w
			if x < x1 {
				x1 = x
			}
			if x > x2 {
				x2 = x
			}
			if y < y1 {
				y1 = y
			}
			if y > y2 {
				y2 = y
			}
		}
		if count < 5 {
			continue
		}
		cropW, cropH := x2-x1+1, y2-y1+1
		// 裁剪 + 颜色蒙版灰度化
		gray := make([]float32, cropW*cropH)
		for y := y1; y <= y2; y++ {
			for x := x1; x <= x2; x++ {
				idx := y*w + x
				p := idx * 3
				if labels[idx] == c {
					gray[(y-y1)*cropW+(x-x1)] = (0.299*float32(rgb[p]) + 0.587*float32(rgb[p+1]) + 0.114*float32(rgb[p+2])) / 255.0
				} else {
					gray[(y-y1)*cropW+(x-x1)] = 1.0
				}
			}
		}
		// 反色 + 二值化
		bin := make([]float32, cropW*cropH)
		for i, g := range gray {
			if 1.0-g > binarizeThresh {
				bin[i] = 1.0
			}
		}
		// 最近邻缩放到 8×6
		var resized [featDim]float32
		for ch := 0; ch < charH; ch++ {
			srcY := ch * cropH / charH
			for cw := 0; cw < charW; cw++ {
				srcX := cw * cropW / charW
				resized[ch*charW+cw] = bin[srcY*cropW+srcX]
			}
		}
		chars = append(chars, charImage{image: resized, bbox: [4]int{x1, y1, x2, y2}})
	}

	sort.Slice(chars, func(i, j int) bool {
		ai := chars[i].bbox[0] + chars[i].bbox[2]
		aj := chars[j].bbox[0] + chars[j].bbox[2]
		return ai < aj
	})
	return chars
}

// classifyChar 对单个字符特征向量分类，返回 (字符索引, 置信度)。
func classifyChar(feature []float32, cents []float32) (int, float64) {
	bestIdx := 0
	bestDist := math.MaxFloat64
	worstDist := 0.0
	for c := 0; c < numClasses; c++ {
		offset := c * featDimWithAR
		pixelDist := 0.0
		for i := 0; i < featDim; i++ {
			d := float64(feature[i] - cents[offset+i])
			pixelDist += d * d
		}
		arDiff := float64(feature[featDim] - cents[offset+featDim])
		dist := pixelDist + arWeight*arDiff*arDiff
		if dist < bestDist {
			bestDist = dist
			bestIdx = c
		}
		if dist > worstDist {
			worstDist = dist
		}
	}
	confidence := 0.0
	if worstDist > 0 {
		confidence = 1.0 - math.Sqrt(bestDist)/math.Sqrt(worstDist)
	}
	return bestIdx, math.Max(0, math.Min(1, confidence))
}

func normalizeAspectRatio(ar float64) float64 {
	return math.Max(0, math.Min(1, ar/maxAspectRatio))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
