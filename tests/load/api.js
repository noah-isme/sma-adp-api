import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const apiErrorRate = new Rate('api_error_rate');

function flag(value) {
  return ['1', 'true', 'yes'].indexOf(String(value || '').toLowerCase()) !== -1;
}

function requiredValue(name, input) {
  const value = String(input || '').trim();
  if (!value) {
    throw new Error(`${name} is required; refusing to run without an explicit target/input`);
  }
  return value;
}

function positiveInteger(value, name) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) {
    throw new Error(`${name} must be a positive integer`);
  }
  return parsed;
}

function optionalEnv(name) {
  return String(__ENV[name] || '').trim();
}

function normalizePrefix(value) {
  const prefix = String(value || '').trim();
  if (!prefix) {
    return '';
  }
  const normalized = prefix.replace(/^\/+|\/+$/g, '');
  return normalized ? `/${normalized}` : '';
}

const rawBaseURL = requiredValue('BASE_URL', __ENV.BASE_URL).replace(/\/+$/, '');
if (!/^https?:\/\/[^\s]+$/i.test(rawBaseURL) || /[?#]/.test(rawBaseURL)) {
  throw new Error('BASE_URL must be an absolute http(s) URL without query parameters');
}

const baseURL = rawBaseURL;
const apiPrefix = normalizePrefix(__ENV.API_PREFIX || '/api/v1');
const smoke = flag(__ENV.SMOKE);
const vus = positiveInteger(__ENV.VUS || (smoke ? '1' : '10'), 'VUS');
const duration = String(__ENV.DURATION || '').trim() || (smoke ? '10s' : '5m');
const accessToken = requiredValue('ACCESS_TOKEN', __ENV.ACCESS_TOKEN).replace(/^Bearer\s*/i, '').trim();

if (!accessToken) {
  throw new Error('ACCESS_TOKEN must contain a JWT value');
}

const termID = optionalEnv('TERM_ID');
const classID = optionalEnv('CLASS_ID');
const studentID = optionalEnv('STUDENT_ID');
const subjectID = optionalEnv('SUBJECT_ID');
const scheduleID = optionalEnv('SCHEDULE_ID');
const reportJobID = optionalEnv('REPORT_JOB_ID');
const runWorkflows = flag(__ENV.RUN_WORKFLOWS);
const includePDF = flag(__ENV.INCLUDE_PDF);
const reportType = optionalEnv('REPORT_TYPE') || 'grades';
const reportFormat = optionalEnv('REPORT_FORMAT') || 'csv';

if (runWorkflows) {
  const missing = ['TERM_ID', 'CLASS_ID', 'SUBJECT_ID', 'TEACHER_ID'].filter((name) => !optionalEnv(name));
  if (missing.length > 0) {
    throw new Error(`RUN_WORKFLOWS=true requires: ${missing.join(', ')}`);
  }
}

export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus,
      duration,
      gracefulStop: '30s',
    },
  },
  thresholds: {
    // Roadmap target: p99 latency below 600 ms.
    http_req_duration: ['p(99)<600'],
    // Roadmap target: less than 1% transport/HTTP failures.
    http_req_failed: ['rate<0.01'],
    // Also count status/check failures for endpoints with a valid HTTP response.
    api_error_rate: ['rate<0.01'],
  },
};

function gatewayURL(path) {
  return `${baseURL}${path}`;
}

function apiURL(path) {
  return `${baseURL}${apiPrefix}${path}`;
}

function request(endpoint, method, url, body, expectedStatuses, authenticated) {
  const headers = { Accept: 'application/json' };
  if (authenticated) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  if (body !== null && body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  const response = http.request(method, url, body || null, {
    headers,
    tags: { endpoint },
  });
  const statuses = Array.isArray(expectedStatuses) ? expectedStatuses : [expectedStatuses];
  const ok = check(response, {
    [`${endpoint} returns an expected status`]: (res) => statuses.indexOf(res.status) !== -1,
  });
  apiErrorRate.add(!ok);
  return { response, ok };
}

function responseData(response) {
  try {
    const payload = response.json();
    return payload && payload.data !== undefined ? payload.data : payload;
  } catch (error) {
    return null;
  }
}

function responseField(response, field) {
  const data = responseData(response);
  if (!data || typeof data !== 'object') {
    return '';
  }
  return data[field] ? String(data[field]) : '';
}

export function setup() {
  if (!runWorkflows) {
    return {};
  }

  // Generate a proposal for timing coverage, but do not save it to the database.
  const schedulePayload = JSON.stringify({
    termId: termID,
    classId: classID,
    timeSlotsPerDay: positiveInteger(__ENV.TIME_SLOTS_PER_DAY || '4', 'TIME_SLOTS_PER_DAY'),
    days: [1, 2, 3, 4, 5],
    subjectLoads: [{
      subjectId: subjectID,
      teacherId: optionalEnv('TEACHER_ID'),
      weeklyCount: positiveInteger(__ENV.WEEKLY_COUNT || '4', 'WEEKLY_COUNT'),
    }],
  });
  const schedule = request(
    'schedule-generate',
    'POST',
    apiURL('/schedule/generate'),
    schedulePayload,
    [200],
    true,
  );
  if (!schedule.ok) {
    throw new Error('Schedule workflow setup failed');
  }

  // Enqueue one report job for the run; VUs only poll its status below.
  const reportPayload = JSON.stringify({
    type: reportType,
    termId: termID,
    classId: classID,
    format: reportFormat,
  });
  const report = request(
    'report-generate',
    'POST',
    apiURL('/reports/generate'),
    reportPayload,
    [202],
    true,
  );
  if (!report.ok) {
    throw new Error('Report workflow setup failed');
  }

  return { reportJobID: responseField(report.response, 'id') };
}

export default function (workflowData) {
  group('public health', () => {
    request('health-public', 'GET', gatewayURL('/health'), null, [200], false);
    request('ready-public', 'GET', gatewayURL('/ready'), null, [200], false);
    request('features-public', 'GET', apiURL('/features'), null, [200], false);
  });

  group('authenticated reads', () => {
    request('auth-me', 'GET', apiURL('/auth/me'), null, [200], true);
    request('terms-list', 'GET', apiURL('/terms?page=1&perPage=20'), null, [200], true);
    request('classes-list', 'GET', apiURL('/classes?page=1&perPage=20'), null, [200], true);
    request('students-list', 'GET', apiURL('/students?page=1&perPage=20'), null, [200], true);
    request('schedules-list', 'GET', apiURL('/schedules?page=1&limit=20'), null, [200], true);
  });

  group('schedule paths', () => {
    if (termID && classID) {
      request(
        'semester-schedule-list',
        'GET',
        apiURL(`/semester-schedule?termId=${encodeURIComponent(termID)}&classId=${encodeURIComponent(classID)}`),
        null,
        [200],
        true,
      );
    }
    if (scheduleID) {
      request('semester-schedule-slots', 'GET', apiURL(`/semester-schedule/${encodeURIComponent(scheduleID)}/slots`), null, [200], true);
    }
    if (includePDF && termID && classID) {
      request(
        'schedule-export-pdf',
        'GET',
        apiURL(`/schedules/export/pdf?class_id=${encodeURIComponent(classID)}&term_id=${encodeURIComponent(termID)}`),
        null,
        [200],
        true,
      );
    }
  });

  group('report paths', () => {
    if (studentID && termID) {
      request(
        'student-report',
        'GET',
        apiURL(`/reports/students/${encodeURIComponent(studentID)}?termId=${encodeURIComponent(termID)}`),
        null,
        [200],
        true,
      );
    }
    if (classID && subjectID && termID) {
      request(
        'class-report',
        'GET',
        apiURL(`/reports/classes/${encodeURIComponent(classID)}?subjectId=${encodeURIComponent(subjectID)}&termId=${encodeURIComponent(termID)}`),
        null,
        [200],
        true,
      );
    }

    const generatedJobID = workflowData && workflowData.reportJobID;
    const jobID = generatedJobID || reportJobID;
    if (jobID) {
      request('report-status', 'GET', apiURL(`/reports/status/${encodeURIComponent(jobID)}`), null, [200], true);
    }
  });

  sleep(smoke ? 0.5 : 1);
}
