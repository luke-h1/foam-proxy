/* eslint-disable no-console */
import { APIGatewayProxyEvent, Context, Handler } from 'aws-lambda';
import * as Sentry from '@sentry/serverless';
import routes from './routes';
import lambdaTimeout from './util/lambdaTimeout';

Sentry.AWSLambda.init({
  dsn: process.env.SENTRY_PROXY_DSN,
  environment: process.env.SENTRY_ENVIRONMENT,
  release: process.env.SENTRY_RELEASE,
  tracesSampleRate: 0.5,
});

export const handler: Handler = Sentry.AWSLambda.wrapHandler(
  async (event: APIGatewayProxyEvent, context: Context) => {
    const { path } = event;

    const { queryStringParameters } = event;

    const queryString = new URLSearchParams(
      queryStringParameters as unknown as string,
    ).toString();

    const url = `https://${event.headers.Host}${event.path}?${queryString}`;

    console.info('origin url ->', url);

    try {
      return await Promise.race([
        routes(path, queryStringParameters, url),
        lambdaTimeout(context),
      ]).then(value => value);
    } catch (e) {
      Sentry.captureException(e);
      return {
        statusCode: 500,
        headers: {
          'Content-Type': 'application/json',
          'Access-Control-Allow-Origin': '*',
        },
        body: {
          error: e,
        },
      };
    }
  },
);
