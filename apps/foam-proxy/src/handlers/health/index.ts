import * as Sentry from '@sentry/serverless';

const healthHandler = () => {
  return Sentry.startSpan(
    {
      name: 'healthHandler',
      op: 'function.health',
    },
    () => {
      return JSON.stringify(
        {
          status: 'OK',
        },
        null,
        2,
      );
    },
  );
};
export default healthHandler;
