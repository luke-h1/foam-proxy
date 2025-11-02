import * as Sentry from '@sentry/aws-serverless';
Sentry.init({
  dsn: process.env.SENTRY_DSN,
  // Adds request headers and IP for users, for more info visit:
  // https://docs.sentry.io/platforms/javascript/guides/aws-lambda/configuration/options/#sendDefaultPii
  sendDefaultPii: true,
  // Add Tracing by setting tracesSampleRate and adding integration
  // Set tracesSampleRate to 1.0 to capture 100% of transactions
  // We recommend adjusting this value in production
  // Learn more at
  // https://docs.sentry.io/platforms/javascript/configuration/options/#traces-sample-rate

  // 20%
  tracesSampleRate: 0.2,
});
