import * as Sentry from '@sentry/serverless';

const pendingHandler = () => {
  return Sentry.startSpan(
    {
      name: 'pendingHandler',
      op: 'function.pending',
    },
    () => {
      return `<html>
          <head>
            <title>Foam - Pending</title>
          </head>
          <body>
            <h1>Your request is pending</h1>
            <p>Please wait while we process your request.</p>
          </body>
        </html>
      `;
    },
  );
};
export default pendingHandler;
