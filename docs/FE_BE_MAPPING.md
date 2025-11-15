# Frontend ↔ Backend Mapping

Referensi cepat jalur menu frontend ke endpoint backend canonical.

| Menu FE                                   | Backend Endpoint                              |
|-------------------------------------------|-----------------------------------------------|
| Dashboard / Admin Overview                | `GET /dashboard`                              |
| Dashboard / Academic Snapshot             | `GET /dashboard/academics`                    |
| Akademik → Kalender                       | `GET /calendar`                               |
| Akademik → Jadwal → Generator             | `POST /schedules/generator`                   |
| Akademik → Jadwal → Preferences           | `GET /schedules/preferences`, `POST /schedules/preferences` |
| Akademik → Jadwal → Simpan Proposal       | `POST /schedule/save` (legacy low-level)      |
| Kehadiran → Ringkasan                     | `GET /attendance`                             |
| Kehadiran → Harian                        | `GET /attendance/daily`                       |
| Arsip → Manajemen Arsip                   | `GET/POST /archives`                          |
| Arsip → Download Arsip                    | `GET /archives/{id}/download`                 |

> Catatan: endpoint `/schedule/generate` dan `/teachers/{id}/preferences` dibiarkan untuk kompatibilitas lama, tetapi FE sebaiknya hanya menggunakan alias di atas.
