# 🚀 Повна інструкція по інтеграції API (Frontend Guide)

**Base URL локально:** `http://localhost:8080`  
**Base URL Cloud Run:** `https://silpo-agent-backend-241107674482.europe-central2.run.app`  
**Формат даних:** `JSON` (крім стрімінгу плану — там `Server-Sent Events / SSE`)  
**Авторизація:** заголовок `Authorization: Bearer <JWT_TOKEN>` для всіх захищених ендпоінтів.

---

## 1. Авторизація та Користувачі (Дві вкладки на фронтенді) 🔐

### 🔹 Вкладка 1: Онбординг (Реєстрація з автогенерацією пароля)
Викликається наприкінці онбордингу, коли користувач вказав ім'я, пошту та авторизувався через Сільпо.
Бекенд **автоматично генерує надійний пароль**, хешує його та **повертає у відповіді** (`generated_password`) одразу для фронтенду! (На пошту нічого не пересилається).

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
> `generated_password` покажіть користувачу в інтерфейсі (модалка/плашка «Ваш згенерований пароль: ...»), щоб він міг зберегти або скопіювати його.

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
Дозволяє користувачу встановити новий пароль у налаштуваннях профілю.  
Старий пароль вводити **не потрібно** (достатньо бути авторизованим через Bearer token).

* **PUT** `/users/me/password`
* **Headers:**
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
* **Body:**
```json
{
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
  "error": "Новий пароль має бути не менше 6 символів"
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

Зберігає щотижневий відгук користувача (оцінки приготованих страв та обрані швидкі теги зауважень).
Ці дані автоматично передаються штучному інтелекту агента під час наступної генерації плану (`/plan/stream`):
- Страви з оцінкою `bad` автоматично виключаються агентом з раціону.
- Обрані теги враховуються агентом для коригування складності, часу приготування, інгредієнтів тощо.

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
  ],
  "created_at": "2026-09-10T02:00:00Z"
}
```

---

### 🔹 2. Отримати останній збережений фідбек
* **GET** `/feedbacks/latest`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):**
```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "user_id": "d7676182-2c97-492f-a057-b33b52142e25",
  "plan_id": "850f40cb-fb2b-4b4c-9bcc-7ec6b8f0825a",
  "dish_ratings": [
    {
      "id": "f-1",
      "title": "Вівсянка на мигдалевому молоці з ягодами та чіа",
      "cookedTimes": 5,
      "timeMinutes": 10,
      "rating": "good"
    }
  ],
  "tags": [
    "Занадто складно готувати"
  ],
  "created_at": "2026-09-10T02:00:00Z"
}
```

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

---

## 7. Архів та Прогрес («Динаміка ваги» та «Витрати на їжу») 📊

Реалізує функціонал екрана «АРХІВ ТА ПРОГРЕС»:
- **Графік динаміки ваги:** фактичні зважування (`T1`..`Tn`), поточна вага, зміна за період, початкова та цільова вага, а також прогнозний пунктир до мети на основі обраного темпу схуднення/набору.
- **Графік витрат на їжу по тижнях:** стовпчикова діаграма витрат (`T1`..`Tn`), ліміт бюджету, середній чек, підрахунок тижнів у межах ліміту та перелік тижнів з перевитратою (наприклад, `1 перевитрата (Т6)`).
- **Експорт у CSV:** завантаження звіту по кнопці «↓ ЕКСПОРТ CSV».

> 🌟 **Повна авто-інтеграція:**
> - Коли користувач змінює вагу в налаштуваннях (`PUT /users/me/settings`), вона автоматично фіксується в історії ваги!
> - Коли генерується новий план (`/plan/stream`), вартість кошика та ліміт бюджету автоматично зберігаються як тижневі витрати!

### 🔹 1. Отримати аналітику прогресу для екрана
* **GET** `/progress?weeks=12`
* **Headers:** `Authorization: Bearer <token>`
* **Query параметри:**
  - `weeks` *(опціонально)*: кількість тижнів для фільтра — `4`, `12` або `all` (за замовчуванням `12`).
* **Response (200 OK):**
```json
{
  "weight": {
    "current_weight": 78.4,
    "start_weight": 82.1,
    "target_weight": 75.0,
    "change_kg": -3.7,
    "period_weeks": 12,
    "history": [
      { "date": "2026-06-18", "week_label": "T1", "weight": 82.1, "is_forecast": false },
      { "date": "2026-07-09", "week_label": "T4", "weight": 80.8, "is_forecast": false },
      { "date": "2026-08-06", "week_label": "T8", "weight": 79.5, "is_forecast": false },
      { "date": "2026-09-03", "week_label": "T12", "weight": 78.4, "is_forecast": false }
    ],
    "forecast": [
      { "date": "2026-10-01", "week_label": "T16", "weight": 76.5, "is_forecast": true },
      { "date": "2026-10-29", "week_label": "T20", "weight": 75.0, "is_forecast": true }
    ]
  },
  "expenses": {
    "weekly_limit": 2000.0,
    "average_spend": 1850.0,
    "weeks_within_limit": 7,
    "total_weeks": 8,
    "overspent_count": 1,
    "overspent_labels": ["Т6"],
    "items": [
      {
        "id": "c1f7a264-e4ad-4b92-80ea-37ab8706d871",
        "week_number": 5,
        "week_label": "Т5",
        "total_cost": 1780.0,
        "budget_limit": 2000.0,
        "is_overspent": false,
        "date": "2026-07-20"
      },
      {
        "id": "e2a8b941-86cc-461d-a94f-561b36914562",
        "week_number": 6,
        "week_label": "Т6",
        "total_cost": 2150.0,
        "budget_limit": 2000.0,
        "is_overspent": true,
        "date": "2026-07-27"
      }
    ]
  }
}
```

---

### 🔹 2. Зафіксувати нову вагу
* **POST** `/progress/weight`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:**
```json
{
  "weight": 78.2,
  "date": "2026-09-10"
}
```
* **Response (201 Created):**
```json
{
  "status": "ok",
  "record": {
    "id": "c86c1257-2e21-4340-97c7-c507a2130ff3",
    "weight": 78.2,
    "recorded_at": "2026-09-10"
  }
}
```

---

### 🔹 3. Додати або скоригувати витрати тижня вручну
* **POST** `/progress/expenses`
* **Headers:**  
  - `Authorization: Bearer <token>`  
  - `Content-Type: application/json`
* **Body:**
```json
{
  "total_cost": 1850.0,
  "budget_limit": 2000.0,
  "week_number": 12,
  "week_label": "Т12",
  "date": "2026-09-10"
}
```
* **Response (201 Created):**
```json
{
  "status": "ok",
  "expense": {
    "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "week_number": 12,
    "week_label": "Т12",
    "total_cost": 1850.0,
    "budget_limit": 2000.0,
    "is_overspent": false,
    "recorded_at": "2026-09-10T00:00:00Z"
  }
}
```

---

### 🔹 4. Завантажити CSV звіт («↓ ЕКСПОРТ CSV»)
* **GET** `/progress/export/csv`
* **Headers:** `Authorization: Bearer <token>`
* **Response (200 OK):** повертає файл `silpo_progress.csv` (UTF-8 з BOM) для скачування та перегляду в Excel.

---

### 💡 Шпаргалка прив'язки даних до UI-елементів екрана «АРХІВ ТА ПРОГРЕС»:

| Елемент на макеті | Звідки брати з відповіді `GET /progress` | Приклад значення |
|---|---|---|
| **Велика цифра ваги** | `data.weight.current_weight` | `78.4` (кг) |
| **Зелений бейдж динаміки** | `data.weight.change_kg` + `data.weight.period_weeks` | `-3.7 кг за 12 тиж` |
| **Лінія «факт» на графіку ваги** | `data.weight.history` | `[{ week_label: "T1", weight: 82.1, date: "..." }]` |
| **Пунктир «прогноз» на графіку** | `data.weight.forecast` | `[{ week_label: "T16", weight: 76.5, date: "..." }]` |
| **Нижній підпис (Старт / Ціль)** | `data.weight.start_weight` та `data.weight.target_weight` | `Старт: 82.1 кг`, `Ціль: 75.0 кг` |
| **Велика цифра середнього чека** | `data.expenses.average_spend` | `1850` (₴ сер. чек / тижд) |
| **Плашка «ЛІМІТ»** | `data.expenses.weekly_limit` | `ЛІМІТ 2 000 ₴` |
| **Стовпчики діаграми витрат** | `data.expenses.items` | кожен стовпчик: `total_cost`, `week_label` (T5..), `is_overspent` |
| **Колір стовпчика** | `item.is_overspent ? 'orange' : 'gray'` (або lime для поточного) | Помаранчевий при перевитраті |
| **Підпис «В межах ліміту»** | `data.expenses.weeks_within_limit` з `data.expenses.total_weeks` | `В межах ліміту: 7 з 8 тиж` |
| **Підпис «Перевитрата»** | `data.expenses.overspent_count` та `data.expenses.overspent_labels` | `1 перевитрата (Т6)` |
| **Кнопка «↓ ЕКСПОРТ CSV»** | `GET /progress/export/csv` | пряме скачування файлу з Bearer-токеном |


