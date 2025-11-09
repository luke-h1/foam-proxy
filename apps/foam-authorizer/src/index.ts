/* eslint-disable prefer-destructuring, no-console */
import {
  APIGatewayRequestAuthorizerEvent,
  APIGatewayAuthorizerResult,
  StatementEffect,
} from 'aws-lambda';
import * as Sentry from '@sentry/serverless';

Sentry.AWSLambda.init({
  dsn: process.env.SENTRY_AUTHORIZER_DSN,
  environment: process.env.SENTRY_ENVIRONMENT || process.env.ENVIRONMENT,
  release: process.env.SENTRY_RELEASE,
  tracesSampleRate: 0.5,
});

export const handler = Sentry.AWSLambda.wrapHandler(
  async (
    event: APIGatewayRequestAuthorizerEvent,
    // eslint-disable-next-line @typescript-eslint/require-await
  ): Promise<APIGatewayAuthorizerResult> => {
    try {
      const apiKey =
        event.headers?.['x-api-key'] ||
        event.queryStringParameters?.['api-key'];

      if (apiKey !== process.env.API_KEY) {
        console.info('deny');

        Sentry.captureEvent({
          message: 'deny',
          level: 'info',
          tags: {
            route: 'authorizer',
          },
          extra: {
            reqUrl: event.methodArn,
          },
        });

        console.info(
          `expected API key ${process.env.API_KEY} but received ${apiKey}`,
        );

        return generatePolicy('user', 'Deny', event.methodArn);
      } else {
        console.info('allow');
        return generatePolicy('user', 'Allow', event.methodArn);
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Error in authorizer:', error);
      Sentry.captureException(error);

      return generatePolicy('user', 'Deny', event.methodArn);
    }
  },
);

const generatePolicy = (
  principalId: string,
  effect: StatementEffect,
  resource: string,
): APIGatewayAuthorizerResult => {
  return {
    principalId,
    policyDocument: {
      Version: '2012-10-17',
      Statement: [
        {
          Action: 'execute-api:Invoke',
          Effect: effect,
          Resource: resource,
        },
      ],
    },
  };
};
