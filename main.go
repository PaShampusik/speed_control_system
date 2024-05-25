package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

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

func main() {
	// Парсим время работы машины
	accessStartTime, _ = time.Parse("15:04:05", "00:00:00")
	accessEndTime, _ = time.Parse("15:04:05", "23:59:00")

	// Создание папки данных, если она отсутсвтует
	if _, err := os.Stat(dataDirectory); os.IsNotExist(err) {
		os.MkdirAll(dataDirectory, 0755)
	}

	// Стартуем наш сервер
	http.HandleFunc("/receive", receiveHandler)
	http.HandleFunc("/query", queryHandler)
	http.ListenAndServe(":8080", nil)
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

	// Создаем карту для хранения данных
	dataMap := make(map[string][]SpeedData)

	// Загружаем существующие данные из файла, если он существует
	filename := fmt.Sprintf("%s/data.json", dataDirectory)
	if _, err := os.Stat(filename); err == nil {
		// Файл существует, загружаем данные
		file, err := os.Open(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		err = json.NewDecoder(file).Decode(&dataMap)
		if err != nil {
			// Проверяем, является ли ошибка io.EOF
			if errors.Is(err, io.EOF) {
				// Инициализируем dataMap как пустую карту
				dataMap = make(map[string][]SpeedData)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else if !os.IsNotExist(err) {
		// Произошла ошибка при проверке существования файла
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Добавляем новые данные в карту
	date := data.DateTime[:10]
	if _, ok := dataMap[date]; !ok {
		// Создаем вложенность для конкретной даты
		dataMap[date] = make([]SpeedData, 1)
	}
	dataMap[date] = append(dataMap[date], data)

	// Сохраняем данные в файл
	jsonData, err := json.Marshal(dataMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Возвращаем успешный ответ
	fmt.Fprintf(w, "Received data: %+v\n", data)
}

// GET handler
// "http://localhost:8080/query?date=2024-05-24&speed_kmph=55.5"
// "http://localhost:8080/query?date=2024-05-24"
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

	// Загружаем существующие данные из файла
	filename := fmt.Sprintf("%s/data.json", dataDirectory)
	var dataMap map[string][]SpeedData
	if _, err := os.Stat(filename); err == nil {
		// Файл существует, загружаем данные
		file, err := os.Open(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		err = json.NewDecoder(file).Decode(&dataMap)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if !os.IsNotExist(err) {
		// Произошла ошибка при проверке существования файла
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ищем данные по запросу
	var responses []SpeedQueryResponse

	if _, ok := dataMap[date]; !ok {
		return
	}

	if speed != -1 {
		// Проверяем, существуют ли данные для запрашиваемой даты и скорости
		// Ищем данные, соответствующие запросу 2.1
		for _, data := range dataMap[date] {
			if data.SpeedKmph > speed {
				// Запрос 2.1: Возврат данных по транспортным средствам, превысившим указанную скорость в указанную дату
				responses = append(responses, SpeedQueryResponse(data))
			}
		}
	} else {
		minSpeed := float64(math.MaxInt64)
		minDate := ""
		minPlate := ""

		maxSpeed := float64(math.MinInt64)
		maxDate := ""
		maxPlate := ""
		// Ищем данные, соответствующие запросу 2.2
		for _, data := range dataMap[date] {
			if data.SpeedKmph > float64(maxSpeed) {
				maxSpeed = data.SpeedKmph
				maxDate = data.DateTime
				maxPlate = data.NumberPlate
			} else if data.SpeedKmph < minSpeed {
				minSpeed = data.SpeedKmph
				minDate = data.DateTime
				minPlate = data.NumberPlate
			}
		}
		// Запрос 2.2: Максимальная и минимальная зафиксированная скорость за указанную дату
		responses = append(responses, SpeedQueryResponse{minDate, minPlate, minSpeed})
		responses = append(responses, SpeedQueryResponse{maxDate, maxPlate, maxSpeed})
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
