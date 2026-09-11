# 🚀 Повна інструкція по інтеграції API (Frontend Guide)

**Base URL локально:** `http://localhost:8080`  
**Base URL Cloud Run:** `https://silpo-agent-backend-241107674482.europe-central2.run.app`  
**Формат даних:** `JSON` (крім стрімінгу плану — там `Server-Sent Events / SSE`)  
**Авторизація:** заголовок `Authorization: Bearer <JWT_TOKEN>` для всіх захищених ендпоінтів.

---

## 1. Авторизація та Користувачі (Дві вкладки на фронтенді) 🔐

### 🔹 Вкладка 1: Онбординг (Реєстрація з генерацією пароля на Email)
Викликається наприкінці онбордингу, коли користувач вказав ім'я, пошту та авторизувався через Сільпо.
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
> 💡 `token` зберігаємо в `localStorage` або `cookies` і додаємо в заголовок `Authorization: Bearer <token>` для всіх наступних запитів.  
> `generated_password` також повертається у відповіді (зручно для показу плашки «Пароль надіслано на вашу пошту: ...»).

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

### 🔹 Отримати профіль поточного юзера
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

### 🔹 Оновити ім'я користувача
* **PUT** `/users/me`
* **Headers:**
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
* **Body:**
```json
{
  "name": "Михайло Новий"
}
```

---

## 2. Параметри та Обмеження (Екран «ПАРАМЕТРИ ТА ОБМЕЖЕННЯ») ⚙️

Цей ендпоінт обслуговує екран із 4 блоками:
1. **Фізичні дані та ціль** (вага, цільова вага, зріст, вік, стать, фокус, темп)
2. **Спортивний режим** (кількість тренувань, розклад по днях, пропуск сьогодні)
3. **Харчові обмеження** (алергени, стоп-продукти, тип харчування)
4. **Бюджет на тиждень** (ліміт витрат, пріоритет акцій, доставка)

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
> 💡 Поле `updated_at` використовується для плашки **«ОНОВЛЕНО 2 ДНІ ТОМУ»**.

---

### 🔹 Зберегти зміни (Кнопка «ЗБЕРЕГТИ ЗМІНИ»)
* **PUT** `/users/me/settings`
* **Headers:**
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
* **Body:**
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

### 🔹 Зберегти/Оновити Silpo токен
Якщо токен не передали під час реєстрації або його треба оновити:
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

## 4. Щотижневий Фідбек («Оцінка страв») 📝

Реалізує функціонал екрану щотижневого відгуку:
- Оцінка страв (`good` / `neutral` / `bad`)
- Швидкі теги зауважень

Бекенд зберігає цей фідбек «як є» — жодного аналізу чи генерації бейджів
рішень на бекенді немає. Оцінки та теги автоматично передаються
core-агенту при наступній генерації плану (`GET /plan/stream`), і саме
агент вирішує, що змінити в раціоні наступного тижня.

### 🔹 1. Зберегти фідбек користувача
* **POST** `/feedbacks`
* **Headers:**
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
* **Body:**
```json
{
  "plan_id": "850f40cb-fb2b-4b4c-9bcc-7ec6b8f0825a",
  "dish_ratings": [
    {
      "id": "f-1",
      "title": "Вівсянка на мигдалевому молоці з ягодами та чіа",
      "cookedTimes": 5,
      "timeMinutes": 10,
      "rating": "good"
    },
    {
      "id": "f-2",
      "title": "Куряче філе з булгуром та печеними овочами",
      "cookedTimes": 4,
      "timeMinutes": 35,
      "rating": "good"
    },
    {
      "id": "f-4",
      "title": "Форель запечена з броколі та лимоном",
      "cookedTimes": 2,
      "timeMinutes": 40,
      "rating": "bad"
    }
  ],
  "tags": [
    "Занадто складно готувати",
    "Набридла курка"
  ]
}
```
* **Response (201 Created):**
```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "user_id": "d7676182-2c97-492f-a057-b33b52142e25",
  "plan_id": "850f40cb-fb2b-4b4c-9bcc-7ec6b8f0825a",
  "dish_ratings": [...],
  "tags": ["Занадто складно готувати", "Набридла курка"],
  "created_at": "2026-09-10T02:00:00Z"
}
```

---

### 🔹 2. Отримати останній збережений фідбек
* **GET** `/feedbacks/latest`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):** останній об'єкт фідбеку з оцінками страв і тегами (без summary/decisions).

---

## 5. Генерація плану (SSE Стрімінг) ⚡

Це основний метод роботи з AI-агентом. Він транслює генерацію плану в реальному часі та автоматично зберігає фінальний результат у БД.

* **GET** `/plan/stream`
* **Headers:** `Authorization: Bearer <token>`
* **Опціональні query параметри:**
  - `note` *(string)* — довільний текст/побажання користувача.
  - `fridge` *(string)* — залишки продуктів у холодильнику через кому (`яйця, молоко, рис`).
  - `plan_id` *(UUID)* — ID попереднього плану (якщо треба скоригувати/уточнити вже згенерований план).
  - `apply` *(boolean, за замовчуванням true)* — чи записувати підібрані товари в реальний кошик Сільпо.

> 🌟 **Повна автоматична інтеграція:**
> - Усі параметри (бюджет, тренування, алергени, стоп-продукти, розклад) беруться з `user_settings`!
> - Останній фідбек користувача (забраковані страви з оцінкою `bad` та вибрані теги) **автоматично враховується** агентом при формуванні нового меню!

### Приклад коду для фронтенду (JavaScript / TypeScript):

Використовуйте бібліотеку `@microsoft/fetch-event-source` (або `fetch`):

```typescript
import { fetchEventSource } from '@microsoft/fetch-event-source';

const url = 'https://silpo-agent-backend-241107674482.europe-central2.run.app/plan/stream';
const token = localStorage.getItem('jwt_token');

await fetchEventSource(url, {
  method: 'GET',
  headers: {
    'Authorization': `Bearer ${token}`
  },
  onmessage(ev) {
    if (!ev.data) return;
    const data = JSON.parse(ev.data);

    // 1. Події виклику інструментів агентом (пошук продуктів, розрахунок БЖВ, перевірка акцій)
    if (data.type === 'tool_call') {
      console.log('🛠️ Агент викликає інструмент:', data.tool, data.args);
    }
    if (data.type === 'tool_result') {
      console.log('✅ Результат інструменту:', data.tool);
    }

    // 2. Стрімінг тексту відповіді по шматочках (друкарська машинка)
    if (data.type === 'token') {
      // Додаємо шматочок тексту до стейту в UI:
      setStreamedText((prev) => prev + (data.text || ''));
    }

    // 3. Фінальний згенерований план і сформований кошик Сільпо
    if (data.type === 'plan') {
      console.log('📄 Повний текст раціону (Markdown):', data.answer);
      console.log('🛒 Товари кошика Сільпо:', data.plan.cart_items);
      console.log('🎯 Цільові калорії та БЖВ:', data.plan.targets);
      console.log('💰 Загальний бюджет:', data.plan.budget_uah);
      setIsDone(true);
    }

    // 4. Помилка (якщо з'єднання з core-агентом перервалося)
    if (data.type === 'error') {
      console.error('❌ Помилка агента:', data.message);
    }
  },
  onerror(err) {
    console.error('Помилка стріму:', err);
  }
});
```

---

## 6. Історія та Перегляд планів 📚

### 🔹 Отримати всі збережені плани
* **GET** `/plans?limit=20&offset=0`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):**
```json
[
  {
    "id": "850f40cb-fb2b-4b4c-9bcc-7ec6b8f0825a",
    "user_id": "d7676182-2c97-492f-a057-b33b52142e25",
    "title": "### ЦІЛІ",
    "content": "{\"answer\":\"### ЦІЛІ...\",\"plan_data\":{\"cart_items\":[...],\"targets\":{...}}}",
    "week_number": 12,
    "week_start_date": "2026-09-07",
    "created_at": "2026-09-06T14:20:21Z"
  }
]
```

> 💡 `week_number` — порядковий номер тижня для цього користувача
> (використовуйте для заголовка «ТИЖДЕНЬ 12» на екрані Фідбек/Тиждень/Архів).
> Рахується автоматично: якщо попередній план був рівно тиждень тому —
> номер збільшується на 1; якщо користувач пропустив один чи більше
> тижнів — рахунок починається заново з 1. `week_start_date` — понеділок
> того календарного тижня, до якого належить план.

> **Порада для UI:** Поле `content` — це JSON-рядок. Зробіть `const parsed = JSON.parse(plan.content)`:
> - `parsed.answer` — готовий красиво відформатований текст (Markdown) з цілями, раціоном на тиждень та таблицею.
> - `parsed.plan_data.cart_items` — масив товарів для відображення або додавання в кошик:
>   - `name`: назва товару
>   - `price`: ціна
>   - `quantity`: кількість
>   - `product_id`: ID товару в системі Сільпо

---

### 🔹 Отримати один конкретний план по ID
* **GET** `/plans/{id}`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):** об'єкт плану.
