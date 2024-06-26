package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SpeedDataOffset struct {
	DateTime    string  `json:"datetime"`
	NumberPlate string  `json:"plate_number"`
	SpeedKmph   float64 `json:"speed_kmph"`
	Offset      int64   `json:"offset"`
}

type SpeedData struct {
	DateTime    string  `json:"datetime"`
	NumberPlate string  `json:"plate_number"`
	SpeedKmph   float64 `json:"speed_kmph"`
}

type SpeedQuery struct {
	Date      string  `json:"date"`
	SpeedKmph float64 `json:"speed_kmph"`
}

type SpeedQueryResponse struct {
	DateTime    string  `json:"datetime"`
	NumberPlate string  `json:"plate_number"`
	SpeedKmph   float64 `json:"speed_kmph"`
}

var dataDirectory = "data"
var accessStartTime time.Time
var accessEndTime time.Time

var offset_map map[string]int64

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Парсим время работы машины
	file, err := os.Open("config.txt")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.SplitN(scanner.Text(), "=", 2)
		if len(line) != 2 {
			continue
		}
		key := strings.TrimSpace(line[0])
		value := strings.TrimSpace(line[1])
		switch key {
		case "start_time":
			accessStartTime, err = time.Parse("15:04", value)
			if err != nil {
				fmt.Println(err)
				return
			}
		case "end_time":
			accessEndTime, err = time.Parse("15:04", value)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}

	// Создание папки данных, если она отсутсвтует
	if _, err := os.Stat(dataDirectory); os.IsNotExist(err) {
		os.MkdirAll(dataDirectory, 0755)
	}

	// Открываем файл
	filename := fmt.Sprintf("%s/map.txt", dataDirectory)

	// Проверяем, существует ли файл, иначе создаем его
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte{}, 0644)
		if err != nil {
			return
		}
	}

	file2, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file2.Close()

	// Создаем сканер для чтения файла строка за строкой
	scanner2 := bufio.NewScanner(file2)

	// Читаем первую строку, содержащую количество элементов
	scanner2.Scan()
	numElements, err := strconv.Atoi(scanner2.Text())
	if err != nil {
		numElements = 0
	}

	// Создаем мапу
	offset_map = make(map[string]int64, numElements)

	// Читаем остальные строки и добавляем их в мапу
	for i := 0; i < numElements; i++ {
		scanner2.Scan()
		line := strings.Split(scanner2.Text(), " ")
		date := line[0]
		offset, err := strconv.ParseInt(line[1], 10, 64)
		if err != nil {
			fmt.Println(err)
			return
		}
		offset_map[date[:10]] = offset
	}

	// Стартуем наш сервер
	http.HandleFunc("/receive", receiveHandler)
	http.HandleFunc("/query", queryHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println(err)
		}
	}()

	<-ctx.Done()

	writeOffsetMapToFile()

	if err := server.Shutdown(context.Background()); err != nil {
		fmt.Println(err)
	}
}

func writeOffsetMapToFile() error {
	filename := fmt.Sprintf("%s/map.txt", dataDirectory)

	// Открываем файл для записи
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Записываем количество элементов в мапе
	if _, err = file.WriteString(fmt.Sprintf("%d\n", len(offset_map))); err != nil {
		return err
	}

	// Записываем пары ключ-значение
	for k, v := range offset_map {
		if _, err = file.WriteString(fmt.Sprintf("%s %d\n", k[:10], v)); err != nil {
			return err
		}
	}

	return nil
}

// Функция парсинга одной записи
func parseLine(offset int64) (SpeedDataOffset, error) {
	filename := fmt.Sprintf("%s/data.txt", dataDirectory)
	file, err := os.Open(filename)
	if err != nil {
		return SpeedDataOffset{}, err
	}
	defer file.Close()

	_, err = file.Seek(offset, 0)
	if err != nil {
		return SpeedDataOffset{}, err
	}

	reader := bufio.NewReader(file)

	line, err := reader.ReadString('\n')
	if err != nil {
		return SpeedDataOffset{}, err
	}

	var data SpeedDataOffset
	lineData := strings.Split(line, " ")
	data.DateTime = lineData[0] + " " + lineData[1]
	data.NumberPlate = lineData[2] + " " + lineData[3]
	data.SpeedKmph, _ = strconv.ParseFloat(lineData[4], 64)
	data.Offset, _ = strconv.ParseInt(lineData[5][:len(lineData[5])-1], 10, 64)

	return data, nil
}

// PUT handler
// Invoke-WebRequest -Method PUT -Headers @{"Content-Type"="application/json"} -Body '{"datetime": "2024-05-25 14:31:25", "plate_number": "1234 PP-7", "speed_kmph": 155.5}' http://localhost:8080/receive
func receiveHandler(w http.ResponseWriter, r *http.Request) {
	// Парсим тело запроса
	var data SpeedData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Добавляем новые данные в файл
	filename := fmt.Sprintf("%s/data.txt", dataDirectory)

	// Проверяем, существует ли файл, иначе создаем его
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte{}, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Открываем файл с флагом os.O_APPEND для того, чтобы не перезаписывать данные
	var str string
	file, _ := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return
	}
	if val, ok := offset_map[data.DateTime[:10]]; ok {
		// Если дата уже есть в мапе, обновляем смещение
		str = fmt.Sprintf("%s %s %.1f %d\n", data.DateTime, data.NumberPlate, data.SpeedKmph, val)
		offset_map[data.DateTime[:10]] = offset
	} else {
		// Если даты нет в мапе, добавляем новую запись
		offset_map[data.DateTime[:10]] = offset
		str = fmt.Sprintf("%s %s %.1f %d\n", data.DateTime, data.NumberPlate, data.SpeedKmph, -1)
	}
	_, err = file.Write([]byte(str))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Возвращаем успешный ответ
	fmt.Fprintf(w, "Received data: %+v\n", data)
}

// GET handler
// "http://localhost:8080/query?date=2024-05-25&speed_kmph=55.5"
// "http://localhost:8080/query?date=2024-05-25"
func queryHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, находится ли текущее время в пределах разрешенного времени доступа.
	now := time.Now()
	if now.Hour() < accessStartTime.Hour() || (now.Hour() == accessStartTime.Hour() && now.Minute() < accessStartTime.Minute()) ||
		now.Hour() > accessEndTime.Hour() || (now.Hour() == accessEndTime.Hour() && now.Minute() > accessEndTime.Minute()) {
		http.Error(w, "Access to data is not allowed at this time", http.StatusForbidden)
		return
	}

	// Парсим тело запроса
	date := r.URL.Query().Get("date")
	speedStr := r.URL.Query().Get("speed_kmph")
	speed, err := strconv.ParseFloat(speedStr, 64)
	if err != nil {
		speed = -1
	}

	var responses []SpeedQueryResponse

	if val, ok := offset_map[date]; ok {
		offset := val
		if speed != -1 {
			for offset != -1 {
				line, _ := parseLine(offset)
				offset = line.Offset
				// Запрос 2.1
				if line.SpeedKmph > speed {
					responses = append(responses, SpeedQueryResponse{line.DateTime, line.NumberPlate, line.SpeedKmph})
				}
			}
		} else {
			minSpeed := float64(math.MaxInt64)
			minDate := ""
			minPlate := ""

			maxSpeed := float64(math.MinInt64)
			maxDate := ""
			maxPlate := ""

			for offset != -1 {
				line, _ := parseLine(offset)
				offset = line.Offset
				if line.SpeedKmph > maxSpeed {
					maxSpeed = line.SpeedKmph
					maxDate = line.DateTime
					maxPlate = line.NumberPlate
				}
				if line.SpeedKmph < minSpeed {
					minSpeed = line.SpeedKmph
					minDate = line.DateTime
					minPlate = line.NumberPlate
				}
			}
			// Запрос 2.2: Максимальная и минимальная зафиксированная скорость за указанную дату
			responses = append(responses, SpeedQueryResponse{minDate, minPlate, minSpeed})
			responses = append(responses, SpeedQueryResponse{maxDate, maxPlate, maxSpeed})
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответы на запросы
	jsonResponses, err := json.Marshal(responses)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponses)
}
