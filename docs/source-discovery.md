# Автоматическое обнаружение источников

## Текущий вертикальный slice

Discovery отделён от получения вакансий. Каталоги GitHub только подсказывают, какие компании проверить; вакансии после регистрации забирает существующая служба `source` через ATS-коннекторы.

Поддерживаемые каталоги по умолчанию:

| Каталог | Репозиторий | Формат |
| --- | --- | --- |
| Remote In Tech | `remoteintech/remote-jobs` | YAML frontmatter в `src/companies/*.md` |
| Established Remote | `yanirs/established-remote` | Markdown-таблица README |
| Global Hiring | `ceolinwill/global-hiring` | Markdown-таблица README |
| Awesome Remote Job | `lukasz-madon/awesome-remote-job` | Ссылки в разделах job boards, companies и freelance |
| European Remote | `EuropeanRemote/european-remote-software-companies` | Markdown-таблица с tech stack и профилем |
| Remote By Default | `RemoteByDefault/remote-software-companies` | Markdown-таблица с tech stack и профилем |
| The Remote Freelancer | `engineerapart/TheRemoteFreelancer` | Markdown-таблица по разделам платформ и job boards |
| Remote Developer Directory | `ugglr/Remote-Developer-jobs-directory` | HTML списки в README по разделам directories/freelancing |
| Awesome GitHub Issues Job Boards | `openings-dev/awesome-github-issues-job-boards` | GitHub repository ссылки из региональных списков |

Backend регистрирует их идемпотентно при старте. Discovery запускается в том же процессе, что и API и другие службы, проверяет каталог раз в сутки и использует GitHub API с default branch, commit SHA и ETag. `GITHUB_TOKEN` необязателен; он передаётся только GitHub API-клиенту и не логируется. `DISCOVERY_WORKER_PARALLELISM` задаёт число одновременно проверяемых кандидатов (1–16, по умолчанию 4).

Найденные компании дедуплицируются по официальному домену или нормализованному имени, а каждая независимая запись каталога сохраняется в `candidate_provenance`. Careers resolver проверяет публичные страницы с защитой от SSRF и распознаёт Greenhouse, Lever и Ashby, а также известные неподдерживаемые ATS. Для Greenhouse/Lever/Ashby проверка использует production connector. Пустой, но корректный ATS board считается действительным. Источник создаётся только после успешного fetch connector; повторное обнаружение связывается с тем же provider key и не меняет operator-поля источника.

Служба `source` остаётся единственным потребителем вакансий. Discovery не фильтрует вакансии по технологиям или региону и не закрывает вакансии самостоятельно. Каталоги job boards и freelance platforms сохраняются отдельно от компаний и пока остаются в `manual_review` до появления подходящего API/RSS-коннектора. Для GitHub Issues каталог проверяет публичность репозитория, включённые Issues, недавнюю активность и открытые Issues с признаками вакансий; затем существующий GitHub connector повторно проверяет feed и импортирует только job-like Issues. Сайты без поддержанного feed/ATS остаются в `manual_review` или `unsupported`; произвольный HTML scraping не выполняется.

## Управление и наблюдаемость

Службу можно запустить/остановить/перезапустить через существующий `/api/v1/admin/services` (имя `discovery`). Состояние каталогов и кандидатов доступно через `GET /api/v1/admin/discovery`; оператор может поставить конкретный каталог на внеплановый запуск через `POST /api/v1/admin/discovery/{id}/run`. Оба маршрута требуют `X-Operator-Token`.

В backend-каталоге:

```sh
go test ./...
go vet ./...
go build ./...
```

Следующий шаг — реализовать connectors для найденных unsupported job boards по реальному coverage report, а также расширить job-level geographic eligibility и employment classification. Discovery сохраняет только company-level сигналы; он не превращает отметку `global hiring` из каталога в remote policy конкретной вакансии.
