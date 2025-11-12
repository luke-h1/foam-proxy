import * as Sentry from '@sentry/serverless';

const proxyHandler = () => {
  return Sentry.startSpan(
    {
      name: 'proxyHandler',
      op: 'function.proxy',
    },
    () => {
      return JSON.stringify({ message: 'redirecting to app' }, null, 2);
    },
  );
};
export default proxyHandler;
