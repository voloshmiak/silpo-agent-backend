# 🚀 Інструкція по інтеграції API (Frontend Guide)

**Base URL локально:** `http://localhost:8080`  
**Base URL Cloud Run:** `https://silpo-agent-backend-241107674482.europe-central2.run.app`  
**Формат даних:** `JSON` (крім стрімінгу плану — там `Server-Sent Events / SSE`)  
**Авторизація:** заголовок `Authorization: Bearer <JWT_TOKEN>` для всіх захищених ендпоінтів.

---

## 1. Авторизація та Користувачі (Дві вкладки на фронтенді) 🔐

### 🔹 Вкладка 1: Онбординг (Реєстрація з генерацією пароля на Email)
Викликається наприкінці онбордингу, коли юзер вказав ім'я, пошту та авторизувався через Сільпо.
Бекенд **автоматично генерує надійний пароль**, хешує його та **відправляє юзеру на вказаний Email** для наступних входів!

* **POST** `/users`
* **Headers:** `Content-Type: application/json`
* **Body:**
```json
{
  "name": "Михайло",
  "email": "user@example.com",
  "silpo_token": "5e1c10ca-0378-4523-abd4-9b5b3cce6084:QtV0Jndg1FHwmBEc:CCaxBjRZE1Ix2zP9sUHfEMOdJOsN5xW5"
}
```
* **Response (201 Created):**
```json
{
  "user": {
    "id": "d7676182-2c97-492f-a057-b33b52142e25",
    "name": "Михайло",
    "email": "user@example.com",
    "created_at": "2026-09-09T20:20:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "generated_password": "k9xP2mQ7aB"
}
```
> 💡 `token` зберігаємо в `localStorage` для наступних запитів. `generated_password` також повертається у відповіді (зручно для тестування або показу плашки «Пароль надіслано на вашу пошту»).

---

### 🔹 Вкладка 2: Вхід за Email та Паролем (Для вже зареєстрованих юзерів)
Використовується на сусідній вкладці справа від онбордингу для повторного входу.

* **POST** `/users/login`
* **Headers:** `Content-Type: application/json`
* **Body:**
```json
{
  "email": "user@example.com",
  "password": "k9xP2mQ7aB"
}
```
* **Response (200 OK):**
```json
{
  "user": {
    "id": "d7676182-2c97-492f-a057-b33b52142e25",
    "name": "Михайло",
    "email": "user@example.com",
    "created_at": "2026-09-09T20:20:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```
* **Response (401 Unauthorized):**
```json
{
  "error": "Невірний email або пароль"
}
```

---

### 🔹 Зміна пароля в профілі користувача
Дозволяє користувачу змінити автозгенерований пароль на свій власний у налаштуваннях профілю.

* **PUT** `/users/me/password`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:**
```json
{
  "old_password": "k9xP2mQ7aB",
  "new_password": "MyNewSecurePassword123"
}
```
* **Response (200 OK):**
```json
{
  "status": "ok",
  "message": "Пароль успішно оновлено"
}
```
* **Response (400 Bad Request):**
```json
{
  "error": "Поточний пароль введено невірно"
}
```

---

### 🔹 Отримати поточного юзера
* **GET** `/users/me`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):**
```json
{
  "id": "d7676182-2c97-492f-a057-b33b52142e25",
  "name": "Михайло",
  "email": "user@example.com",
  "created_at": "2026-09-09T20:20:00Z"
}
```

---

## 2. Параметри та Обмеження (Екран «ПАРАМЕТРИ ТА ОБМЕЖЕННЯ») ⚙️

### 🔹 Отримати всі збережені параметри (витягнути з БД)
* **GET** `/users/me/settings`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):**
```json
{
  "user_id": "d7676182-2c97-492f-a057-b33b52142e25",
  "weight": 78.4,
  "target_weight": 72.5,
  "height": 182.0,
  "age": 29,
  "sex": "чол.",
  "focus": "Схуднення",
  "weekly_pace": -0.6,
  "workouts_per_week": 4,
  "workout_schedule": {
    "ПН": "силові",
    "ВТ": "кардіо",
    "ЧТ": "силові",
    "СБ": "силові"
  },
  "missed_workout_today": false,
  "allergens": ["лактоза", "горіхи"],
  "excluded_products": ["гриби", "кінза", "печінка"],
  "diet_type": "БЕЗ ОБМЕЖЕНЬ",
  "weekly_budget": 2000.0,
  "promo_priority": "Високий",
  "delivery_included": true,
  "updated_at": "2026-09-08T15:30:00Z"
}
```

---

### 🔹 Зберегти зміни (Кнопка «ЗБЕРЕГТИ ЗМІНИ»)
* **PUT** `/users/me/settings`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:** передаються збережені поля:
```json
{
  "weight": 78.4,
  "target_weight": 72.5,
  "height": 182.0,
  "age": 29,
  "sex": "чол.",
  "focus": "Схуднення",
  "weekly_pace": -0.6,
  "workouts_per_week": 4,
  "workout_schedule": {
    "ПН": "силові",
    "ВТ": "кардіо",
    "ЧТ": "силові",
    "СБ": "силові"
  },
  "missed_workout_today": false,
  "allergens": ["лактоза", "горіхи"],
  "excluded_products": ["гриби", "кінза", "печінка"],
  "diet_type": "БЕЗ ОБМЕЖЕНЬ",
  "weekly_budget": 2000.0,
  "promo_priority": "Високий",
  "delivery_included": true
}
```

---

## 3. Генерація плану (SSE Стрімінг) ⚡

* **GET** `/plan/stream`
* **Headers:** `Authorization: Bearer <token>`
*(Всі параметри: бюджет, тренування, алергени та фізичні показники автоматично підтягуються з `user_settings`!)*

---

## 4. Історія планів

* **GET** `/plans?limit=20&offset=0` — список збережених планів.
* **GET** `/plans/:id` — конкретний план.
