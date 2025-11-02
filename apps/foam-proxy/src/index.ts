/* eslint-disable no-console */
import * as newrelic from 'newrelic';
import { APIGatewayProxyEvent, Context, Handler } from 'aws-lambda';
import routes from './routes';
import lambdaTimeout from './util/lambdaTimeout';

const lambdaHandler: Handler = async (
  event: APIGatewayProxyEvent,
  context: Context,
) => {
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
};

export const handler = newrelic.setLambdaHandler(lambdaHandler);
