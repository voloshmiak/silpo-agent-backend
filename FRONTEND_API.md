# 🚀 Інструкція по інтеграції API (Frontend Guide)

**Base URL локально:** `http://localhost:8080`  
**Base URL Cloud Run:** `https://silpo-agent-backend-241107674482.europe-central2.run.app`  
**Формат даних:** `JSON` (крім стрімінгу плану — там `Server-Sent Events / SSE`)  
**Авторизація:** заголовок `Authorization: Bearer <JWT_TOKEN>` для всіх захищених ендпоінтів.

---

## 1. Користувачі

### 🔹 Створити юзера / Отримати JWT
Викликається при першому вході або реєстрації.
* **POST** `/users`
* **Body:**
```json
{
  "name": "Михайло",
  "silpo_token": "5e1c10ca-0378-4523-abd4-9b5b3cce6084:QtV0Jndg1FHwmBEc:CCaxBjRZE1Ix2zP9sUHfEMOdJOsN5xW5"
}
```
*(поле `silpo_token` опціональне — можна передати одразу при реєстрації, або зберегти пізніше через `POST /users/me/silpo-token`)*
* **Response (201 Created):**
```json
{
  "user": {
    "id": "d7676182-2c97-492f-a057-b33b52142e25",
    "name": "Михайло",
    "created_at": "2026-09-06T14:20:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

---

### 🔹 Отримати поточного юзера
* **GET** `/users/me`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):** `{"id": "...", "name": "Михайло", "created_at": "..."}`

---

## 2. Параметри та Обмеження (Екран «ПАРАМЕТРИ ТА ОБМЕЖЕННЯ») ⚙️

Цей ендпоінт містить усі 4 блоки з інтерфейсу:
1. **Фізичні дані та ціль**
2. **Спортивний режим**
3. **Харчові обмеження**
4. **Бюджет на тиждень**

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
> 💡 Поле `updated_at` використовується для плашки **«ОНОВЛЕНО Х ДНІВ ТОМУ»**.

---

### 🔹 Зберегти зміни (Кнопка «ЗБЕРЕГТИ ЗМІНИ»)
* **PUT** `/users/me/settings`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:** передаються всі або змінені параметри з точними типами даних:
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
* **Response (200 OK):** повертає оновлений об'єкт `settings` із новим `updated_at`.

---

## 3. Silpo MCP Токен

### 🔹 Зберегти Silpo токен
* **POST** `/users/me/silpo-token`
* **Headers:** `Authorization: Bearer <token>`
* **Body:**
```json
{
  "access_token": "5e1c10ca-0378-4523-abd4-9b5b3cce6084:QtV0Jndg1FHwmBEc:CCaxBjRZE1Ix2zP9sUHfEMOdJOsN5xW5"
}
```
* **Response (200 OK):** `{"status": "ok"}`

---

## 4. Генерація плану (SSE Стрімінг) ⚡

Бекенд **автоматично підтягує всі збережені параметри з БД** (вагу, зріст, алергени, стоп-продукти, розклад та бюджет), тому в URL більше не обов'язково передавати довгий список параметрів!

* **GET** `/plan/stream`
* **Headers:** `Authorization: Bearer <token>`
* **Опціональні query параметри (якщо треба тимчасово перевизначити збережені в базі):**
  - `budget_uah` *(float)* — за замовчуванням береться зі збереженого `weekly_budget`.
  - `workouts` *(int)* — за замовчуванням береться зі збереженого `workouts_per_week`.
  - `note` *(string)* — додаткова нотатка (алергени та стоп-продукти з налаштувань додаються сюди автоматично!).
  - `fridge` *(string)* — продукти в холодильнику через кому.
  - `plan_id` *(UUID)* — якщо треба скоригувати конкретний попередній план.

Події SSE:
- `{"type": "tool_call", ...}` — виклик інструментів.
- `{"type": "token", "text": "..."}` — стрімінг тексту відповіді.
- `{"type": "plan", "answer": "...", "plan": {...}}` — фінальний план і кошик.

---

## 5. Історія планів

* **GET** `/plans?limit=20&offset=0` — список усіх згенерованих планів.
* **GET** `/plans/:id` — конкретний план.
