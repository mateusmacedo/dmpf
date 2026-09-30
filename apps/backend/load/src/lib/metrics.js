import { Counter, Rate, Trend } from 'k6/metrics';

export const admissionRejections = new Rate('admission_rejections');
export const contextAdmissionRejections = new Counter('context_admission_rejections');
export const reservationConvergence = new Trend('reservation_convergence', true);
export const reservationConvergenceTimeouts = new Counter('reservation_convergence_timeouts');
