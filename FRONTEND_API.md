# 🚀 Інструкція по інтеграції API (Frontend Guide)

**Base URL локально:** `http://localhost:8080`  
**Формат даних:** `JSON` (крім стрімінгу плану — там `Server-Sent Events / SSE`)  
**Авторизація:** заголовок `Authorization: Bearer <JWT_TOKEN>` для всіх захищених ендпоінтів.

---

## 1. Користувачі та Профіль

### 🔹 Створити юзера / Отримати JWT
Викликається при першому вході або реєстрації.
* **POST** `/users`
* **Headers:** `Content-Type: application/json`
* **Body:**
```json
{
  "name": "Іван"
}
```
* **Response (201 Created):**
```json
{
  "user": {
    "id": "d7676182-2c97-492f-a057-b33b52142e25",
    "name": "Іван",
    "weight": 0,
    "height": 0,
    "created_at": "2026-09-06T14:20:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```
> ⚠️ **Важливо:** Збережіть `token` у `localStorage` або `cookies` і додавайте його в заголовок `Authorization: Bearer <token>` для всіх наступних запитів.

---

### 🔹 Отримати профіль
* **GET** `/users/me`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):** повертає об'єкт користувача.

---

### 🔹 Оновити параметри (вага, зріст)
Потрібно для коректного розрахунку калорій агентом.
* **PUT** `/users/me`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:**
```json
{
  "name": "Іван",
  "weight": 80,
  "height": 180
}
```

---

## 2. Silpo MCP Токен

### 🔹 Зберегти Silpo токен користувача
Коли користувач авторизується у Сільпо, надішліть його токен на бекенд (бекенд сам зберігає його в БД та оновлює).
* **POST** `/users/me/silpo-token`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:**
```json
{
  "access_token": "5e1c10ca-0378-4523-abd4-9b5b3cce6084:QtV0Jndg1FHwmBEc:CCaxBjRZE1Ix2zP9sUHfEMOdJOsN5xW5",
  "refresh_token": "опціонально_якщо_є"
}
```
* **Response (200 OK):** `{"status": "ok"}`

---

## 3. Генерація плану (SSE Стрімінг) ⚡

Це основний метод для роботи з AI-агентом. Він транслює генерацію плану в реальному часі.

* **GET** `/plan/stream`
* **Headers:** `Authorization: Bearer <token>`
* **Query параметри:**
  - `budget_uah` *(обов'язковий, float)* — бюджет на закупку в гривнях, наприклад `500`.
  - `workouts` *(int, за замовчуванням 0)* — кількість тренувань на тиждень.
  - `sex` *(string: "male" або "female")* — стать.
  - `age` *(int)* — вік.
  - `note` *(string)* — додаткові побажання («хочу схуднути», «без лактози»).
  - `fridge` *(string)* — продукти, що вже є вдома, через кому («яйця, молоко, сир»).
  - `target_weight` *(float)* — цільова вага.
  - `plan_id` *(UUID, опціонально)* — передається, якщо треба **уточнити/скоригувати** вже існуючий план.

### Приклад запиту з браузера (JavaScript / TypeScript):

Оскільки стандартний `EventSource` у браузері не підтримує кастомні заголовки `Authorization`, рекомендується використовувати бібліотеку `@microsoft/fetch-event-source` або звичайний `fetch`:

```javascript
import { fetchEventSource } from '@microsoft/fetch-event-source';

const url = 'http://localhost:8080/plan/stream?budget_uah=500&workouts=3&sex=male&age=25';

await fetchEventSource(url, {
  headers: {
    'Authorization': `Bearer ${token}`
  },
  onmessage(ev) {
    const data = JSON.parse(ev.data);

    // 1. Агент викликає інструмент (пошук продуктів, розрахунок БЖВ тощо)
    if (data.type === 'tool_call') {
      console.log('Агент думає:', data.tool, data.args);
    }

    // 2. Стрімінг тексту відповіді по шматочках
    if (data.type === 'token') {
      console.log('Шматочок тексту:', data.text);
    }

    // 3. Фінальний згенерований план і сформований кошик
    if (data.type === 'plan') {
      console.log('Повний текст (Markdown):', data.answer);
      console.log('Товари для кошика:', data.plan.cart_items);
      console.log('Цільові калорії та БЖВ:', data.plan.targets);
    }
  },
  onerror(err) {
    console.error('Помилка стріму:', err);
  }
});
```

> 💡 **Зверніть увагу:** Бекенд сам автоматично зберігає весь згенерований план у базу даних після закінчення стріму! Фронтенду нічого додатково зберігати не треба.

---

## 4. Історія та Перегляд планів

### 🔹 Отримати всі збережені плани
* **GET** `/plans?limit=20&offset=0`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):**
```json
[
  {
    "id": "850f40cb-fb2b-4b4c-9bcc-7ec6b8f0825a",
    "user_id": "d7676182-2c97-492f-a057-b33b52142e25",
    "title": "Plan 2026-09-06 14:20",
    "content": "{\"answer\":\"### ЦІЛІ...\",\"plan_data\":{\"cart_items\":[...],\"targets\":{...}}}",
    "created_at": "2026-09-06T14:20:21Z"
  }
]
```
> **Порада для UI:** Поле `content` — це JSON-рядок. Зробіть `JSON.parse(plan.content)`:
> - `content.answer` — готовий красиво відформатований текст (Markdown) з цілями, меню на 7 днів та таблицею.
> - `content.plan_data.cart_items` — готовий масив товарів із цінами, назвами та ID для додавання в кошик Silpo.

---

### 🔹 Отримати один конкретний план по ID
* **GET** `/plans/{id}`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):** об'єкт плану.
