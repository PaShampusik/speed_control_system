# speed_control_system

* Invoke-WebRequest -Method PUT -Headers @{"Content-Type"="application/json"} -Body '{"datetime": "2024-05-25 14:31:25", "plate_number": "1234 PP-7", "speed_kmph": 155.5}' http://localhost:8080/receive
* Invoke-WebRequest -Method GET -Uri "http://localhost:8080/query?date=2024-05-24&speed_kmph=55.5"
* Invoke-WebRequest -Method GET -Uri "http://localhost:8080/query?date=2024-05-24"
