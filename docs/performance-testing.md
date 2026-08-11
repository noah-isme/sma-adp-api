# Performance testing Go API

Runbook ini menjalankan harness k6 di `tests/load/api.js`. Target default adalah
environment lokal atau staging yang memang disiapkan untuk pengujian. Harness
menolak proses tanpa `BASE_URL` dan `ACCESS_TOKEN`, tidak melakukan login, dan
tidak menyimpan token atau URL target di repository.

## Cakupan

- Health publik: `GET /health`, `GET /ready`, dan `GET /api/v1/features`.
- Read terautentikasi: `/auth/me`, serta daftar terms, classes, students, dan schedules.
- Path schedule opsional: semester schedule, slot schedule, dan export PDF.
- Path report opsional: student/class report dan status report job.
- Workflow yang mengubah state (`POST /schedule/generate` dan `POST /reports/generate`)
  hanya dijalankan jika `RUN_WORKFLOWS=true`. Setup hanya membuat satu report job
  dan tidak menyimpan proposal schedule.

## Input konfigurasi

`BASE_URL` adalah origin gateway, tanpa suffix `/api/v1`; prefix API dapat diubah
dengan `API_PREFIX` bila deployment menggunakannya.

| Input | Wajib | Default | Keterangan |
| --- | --- | --- | --- |
| `BASE_URL` | Ya | — | URL lokal/staging yang sengaja dipilih |
| `ACCESS_TOKEN` | Ya | — | JWT mentah atau nilai dengan prefix `Bearer`; gunakan token read untuk default, admin-capable untuk workflow |
| `VUS` | Tidak | `10` (`1` pada smoke) | Jumlah virtual users |
| `DURATION` | Tidak | `5m` (`10s` pada smoke) | Durasi k6, misalnya `30s`, `5m`, atau `1h` |
| `SMOKE` | Tidak | `false` | `true`/`1` memakai 1 VU dan 10 detik kecuali `VUS`/`DURATION` di-override |
| `API_PREFIX` | Tidak | `/api/v1` | Prefix route API |
| `TERM_ID`, `CLASS_ID` | Tidak | — | Mengaktifkan pembacaan semester schedule |
| `STUDENT_ID`, `SUBJECT_ID`, `SCHEDULE_ID`, `REPORT_JOB_ID` | Tidak | — | Mengaktifkan path report/slot terkait |
| `INCLUDE_PDF` | Tidak | `false` | Opt-in export PDF; lebih berat daripada read JSON |
| `RUN_WORKFLOWS` | Tidak | `false` | Opt-in schedule generation dan satu report enqueue |
| `TEACHER_ID` | Saat workflow | — | ID teacher untuk payload generator |

Nilai ID data di atas adalah identifier fixture pada target test, bukan nilai
yang ditulis di script. `RUN_WORKFLOWS=true` juga membutuhkan `TERM_ID`,
`CLASS_ID`, `SUBJECT_ID`, dan `TEACHER_ID`; `REPORT_TYPE`, `REPORT_FORMAT`,
`TIME_SLOTS_PER_DAY`, dan `WEEKLY_COUNT` dapat diubah bila fixture memerlukannya.

## Menjalankan

Dari direktori `sma-adp-api`:

```bash
# Smoke: verifikasi cepat sebelum load test.
BASE_URL="$BASE_URL" ACCESS_TOKEN="$ACCESS_TOKEN" SMOKE=true \
  k6 run tests/load/api.js

# Baseline read-only dengan target dan token yang sudah disediakan oleh shell/secret manager.
BASE_URL="$BASE_URL" ACCESS_TOKEN="$ACCESS_TOKEN" VUS=10 DURATION=5m \
  k6 run tests/load/api.js
```

Untuk menjalankan path berbasis fixture, tambahkan hanya ID milik environment
test tersebut:

```bash
BASE_URL="$BASE_URL" ACCESS_TOKEN="$ACCESS_TOKEN" \
  TERM_ID="$TERM_ID" CLASS_ID="$CLASS_ID" STUDENT_ID="$STUDENT_ID" \
  SUBJECT_ID="$SUBJECT_ID" SCHEDULE_ID="$SCHEDULE_ID" \
  VUS=5 DURATION=2m k6 run tests/load/api.js
```

Workflow state-changing harus dijalankan hanya pada environment yang disetujui
untuk itu. Harness membuat satu proposal schedule untuk pengukuran dan satu
report job untuk polling; ia tidak memanggil endpoint save/delete.

```bash
BASE_URL="$BASE_URL" ACCESS_TOKEN="$ACCESS_TOKEN" RUN_WORKFLOWS=true \
  TERM_ID="$TERM_ID" CLASS_ID="$CLASS_ID" SUBJECT_ID="$SUBJECT_ID" \
  TEACHER_ID="$TEACHER_ID" SMOKE=true k6 run tests/load/api.js
```

## Threshold dan guardrail

Threshold k6 mengikuti roadmap:

- `http_req_duration`: p99 `< 600 ms`.
- `http_req_failed`: error transport/HTTP `< 1%`.
- `api_error_rate`: status atau check endpoint yang gagal `< 1%`.

Gunakan token test dengan hak minimum, token admin hanya untuk workflow, pastikan
`BASE_URL` bukan production, dan mulai dari smoke. Jangan memakai `--http-debug`
karena header Authorization dapat masuk ke output. Hentikan pengujian bila error
rate atau p99 melewati threshold; simpan ringkasan k6 bersama catatan tanggal,
commit, VU, durasi, dan fixture yang dipakai untuk membentuk baseline.

## Template bukti baseline

Simpan ringkasan hasil di catatan staging atau sistem observability yang dipakai
tim. Jangan memasukkan token, cookie, atau URL yang mengandung kredensial.

```text
Tanggal / zona waktu:
Commit:
Target staging:
VUS / durasi:
Fixture yang digunakan:
http_req_duration p99:
http_req_failed:
api_error_rate:
Hasil threshold: PASS | FAIL
Catatan anomali / tindak lanjut:
```

Baseline belum dianggap selesai sampai hasil ini ditinjau bersama pemilik
service dan dilampirkan pada catatan rilis.
