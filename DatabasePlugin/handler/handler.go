package handler

import (
	"bufio"
	"bytes"
	"context"
	"database/pkg"
	"database/send"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DataChannel = make(chan HandleTask, 100)

const maxConcurrentIO = 10

var ioSem = make(chan struct{}, maxConcurrentIO)

type Handler interface {
	Handle(HandleTask) (send.ToTrain, error)
	Get(lastID, limit int) ([]HandleTask, int, error)
	Count() (int64, error)
	DisConnect() error
}

type HandleTask struct {
	Type  string
	Path  string
	DType string
}

type Select struct {
	Field    string
	Operator string
	Value    string
}

type DefaultHandler struct {
	wg       *sync.WaitGroup
	Account  string
	Password string
	DBType   string
	selector []Select
	db       *gorm.DB
	ctx      context.Context
	Id       string
	mux      *sync.RWMutex
	file     *os.File
}

func NewDefaultHandler(account string, password string, dbType string, s []Select, ctx context.Context, id string, mux *sync.RWMutex, file *os.File) (*DefaultHandler, error) {
	dsn := account + ":" + password + "@tcp(127.0.0.1:3306)/data?parseTime=true"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &DefaultHandler{
		wg:       &sync.WaitGroup{},
		Account:  account,
		Password: password,
		DBType:   dbType,
		selector: s,
		db:       db,
		ctx:      ctx,
		Id:       id,
		mux:      mux,
		file:     file,
	}, nil
}

func (h *DefaultHandler) Get(lastID, limit int) ([]HandleTask, int, error) {
	h.wg.Add(1)
	defer h.wg.Done()
	select {
	case <-h.ctx.Done():
		return nil, lastID, h.ctx.Err()
	default:
		db, err := SelectFunc(h.db, h.selector)
		if err != nil {
			pkg.MuxLog(h.file, err, h.Id, false, h.mux)
			return nil, lastID, err
		}
		var datas []Data
		db.Where("ID > ?", lastID).Order("ID ASC").Limit(limit).Find(&datas)
		re := make([]HandleTask, 0, limit)
		cursor := lastID
		for _, data := range datas {
			re = append(re, HandleTask{
				Type:  data.Type,
				Path:  data.Path,
				DType: data.Dtype,
			})
			if data.ID > cursor {
				cursor = data.ID
			}
		}
		return re, cursor, nil
	}
}

func (h *DefaultHandler) Count() (int64, error) {
	h.wg.Add(1)
	defer h.wg.Done()
	select {
	case <-h.ctx.Done():
		return 0, h.ctx.Err()
	default:
	}
	db, err := SelectFunc(h.db, h.selector)
	if err != nil {
		return 0, err
	}
	var total int64
	if err := db.Model(&Data{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (h *DefaultHandler) Handle(t HandleTask) (send.ToTrain, error) {
	ioSem <- struct{}{}
	defer func() { <-ioSem }()

	re := send.ToTrain{
		Type:   t.Type,
		DType:  t.DType,
		Inputs: make(map[string]*send.Tensor),
	}
	switch t.DType {
	case "jpg", "jpeg", "png":
		tensor, err := imageToTensor(t.Path)
		if err != nil {
			pkg.MuxLog(h.file, err, h.Id, false, h.mux)
			return send.ToTrain{}, err
		}
		re.Inputs[t.DType] = tensor
	case "txt":
		tensor, err := textToTensor(t.Path)
		if err != nil {
			pkg.MuxLog(h.file, err, h.Id, false, h.mux)
			return send.ToTrain{}, err
		}
		re.Inputs[t.DType] = tensor
	case "json":
		images, words, err := parseJSON(t.Path)
		if err != nil {
			pkg.MuxLog(h.file, err, h.Id, false, h.mux)
			return send.ToTrain{}, err
		}
		imgTensor, err := imagesToTensor(images)
		if err != nil {
			pkg.MuxLog(h.file, err, h.Id, false, h.mux)
			return send.ToTrain{}, err
		}
		wordTensor := wordsToTensor(words)
		re.Inputs["image"] = imgTensor
		re.Inputs["word"] = wordTensor
	default:
		err := fmt.Errorf("unsupported dtype: %s", t.DType)
		pkg.MuxLog(h.file, err, h.Id, false, h.mux)
		return send.ToTrain{}, err
	}
	return re, nil
}

const (
	imageWidth  = 224
	imageHeight = 224
)

func writeImageTensor(dst *bytes.Buffer, data []float32, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	b := img.Bounds()
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	pix := src.Pix
	stride := src.Stride

	xs := make([]int, imageWidth)
	for x := 0; x < imageWidth; x++ {
		xs[x] = x * b.Dx() / imageWidth
	}
	ys := make([]int, imageHeight)
	for y := 0; y < imageHeight; y++ {
		ys[y] = y * b.Dy() / imageHeight
	}

	i := 0
	for y := 0; y < imageHeight; y++ {
		row := ys[y] * stride
		for x := 0; x < imageWidth; x++ {
			p := row + xs[x]*4
			data[i] = float32(pix[p]) / 255
			data[i+1] = float32(pix[p+1]) / 255
			data[i+2] = float32(pix[p+2]) / 255
			i += 3
		}
	}
	return binary.Write(dst, binary.LittleEndian, data)
}

func imageToTensor(path string) (*send.Tensor, error) {
	buf := new(bytes.Buffer)
	data := make([]float32, imageWidth*imageHeight*3)
	if err := writeImageTensor(buf, data, path); err != nil {
		return nil, err
	}
	return &send.Tensor{
		Dim:           []int64{imageHeight, imageWidth, 3},
		TensorContent: buf.Bytes(),
	}, nil
}

func textToTensor(path string) (*send.Tensor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vals := make([]float32, len(data))
	for i, b := range data {
		vals[i] = float32(b)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, vals); err != nil {
		return nil, err
	}
	return &send.Tensor{
		Dim:           []int64{int64(len(data))},
		TensorContent: buf.Bytes(),
	}, nil
}

type jsonSample struct {
	Image string `json:"image"`
	Word  string `json:"word"`
}

func parseJSON(path string) ([]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var images, words []string
	var s jsonSample
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			return nil, nil, err
		}
		images = append(images, s.Image)
		words = append(words, s.Word)
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	return images, words, nil
}

func imagesToTensor(paths []string) (*send.Tensor, error) {
	buf := new(bytes.Buffer)
	data := make([]float32, imageWidth*imageHeight*3)
	for _, p := range paths {
		if err := writeImageTensor(buf, data, p); err != nil {
			return nil, err
		}
	}
	return &send.Tensor{
		Dim:           []int64{int64(len(paths)), imageHeight, imageWidth, 3},
		TensorContent: buf.Bytes(),
	}, nil
}

func wordsToTensor(words []string) *send.Tensor {
	maxLen := 0
	for _, w := range words {
		if len(w) > maxLen {
			maxLen = len(w)
		}
	}
	data := make([]float32, len(words)*maxLen)
	for i, w := range words {
		for j := 0; j < len(w); j++ {
			data[i*maxLen+j] = float32(w[j])
		}
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, data)
	return &send.Tensor{
		Dim:           []int64{int64(len(words)), int64(maxLen)},
		TensorContent: buf.Bytes(),
	}
}

func (h *DefaultHandler) DisConnect() error {
	close(ioSem)
	close(DataChannel)
	sqldb, err := h.db.DB()
	if err != nil {
		return err
	}
	h.wg.Wait()
	sqldb.Close()
	return nil
}
