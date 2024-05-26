# speed_control_system

SPACE COMPLEXITY OF GET REQUEST: O(k)
TIME COMPLEXITY OF GET REQUEST: O(n/k)

где k — количество уникальных дат,
n — общее количество записей в файл данных

    Изначально была идея реализовать работу с файлом данных, как с деревом отрезков, т.е. хранить сами данные в виде дерева отрезков в самом файле, однако, очень сильно останавливало то, что, скорее всего, в подобную систему происходит сильно больше вставок, чем обходов. Даже, если бы обходы и вставки были за log(n), что сильно бы ускоряло поиск записей, блокировки программы для перестройки дерева во время вставки ужасно бы ухудшили произоводительность.

    Интересно выглядела идея с построением префиксных сумм, для бысрого нахождения минимальной и максимальной скорости на заданную дату, но, учитывая, что для второго вида запроса, когда нам нужно найти все записи, где скорость превышает данную, префиксные суммы бесполезны, т.к. мы заранее не знаем скорость, для которой строить префиксную сумму, а перестраивать ее каждый раз при запросе еще более накладно по времени, чем за линию.

    Можно было бы просто быстро за линию читать весь файл, но линия в рамках файла банными на пару гигабайт является очень долгим процессом(больше 10) секунд по бенчмаркам из сети интернет.

    В ходе анализа удалось найти оптимальное решение с использованием памяти О(k), где k — количество уникальных дат в файле данных, и асимптотической сложностью O(m), где m — количество записей, сделанных в конкретную дату. Если мы будем рассматривать все дни равнозначными в плане количества записей, то без потери общности асимптотическую сложность можно записать как О(n/k), где n — общее количество записей в файле данных, k — количество различных дат в файле данных.
 Распишу подробно, в чем состоит метод считывания. У нас есть основной файл с данными, куда записываются все данные, не сортируя записи, просто в конец файла, формат записи: YYYY.MM.DD HH.MM.SS PLATE_NUMBER SPEED OFFSET. У нас есть отдельный файл для инициализации map[string]int, где string — конкретная дата в виде YYYY.MM.D, а int является байтовым смещением в файле данных, которое указывает на последнюю запись, содержащуюся для этой даты. Т.е. содержимое файла map.txt выглядит так: 2024.05.25 1237472 2024.05.26 363722 и так далее. В итоге мы имеем указатели на последние записи для всех дат, в самой же записи внутри файла данных последним элементом является OFFSET, который является байтовым смещением от начала файла на предыдущую запись с такой же датой. В итоге map поддерживает k записей, что формирует расход памяти O(k). Для выполнения запросов 2.1 и 2.2 нам необходимо пройтись по всем записям даты, а это можно представить как прохождение связного списка размера m.
	Вероятно, можно найти более оптимальное решение данной задачи, но мне это не удалось.

Вот команды для проверки работы сервера:

* Invoke-WebRequest -Method PUT -Headers @{"Content-Type"="application/json"} -Body '{"datetime": "2024-05-25 14:31:25", "plate_number": "1234 PP-7", "speed_kmph": 155.5}' http://localhost:8080/receive
* Invoke-WebRequest -Method GET -Uri "http://localhost:8080/query?date=2024-05-25&speed_kmph=55.5"
* Invoke-WebRequest -Method GET -Uri "http://localhost:8080/query?date=2024-05-24"

Первая команда выполняет PUT запрос, который создвет в файле данных новую запись.
Вторая и третья команды выполняют GET  запросы, которые соответствуют заданию.
