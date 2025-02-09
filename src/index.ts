import { APIGatewayProxyEvent, Context, Handler } from 'aws-lambda';
import routes from './routes';
import lambdaTimeout from './util/lambdaTimeout';

export const handler: Handler = async (
  event: APIGatewayProxyEvent,
  context: Context,
) => {
  const path =
    // path can either be the last part of the path or the routeKey
    // depending on whether the function is executed from aws or a http call comes thru from the http gateway
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    event.requestContext?.path?.split('/').pop() ??
    // @ts-expect-error missing aws-lambda types
    event.routeKey ??
    // @ts-expect-error missing aws-lambda types
    event.rawPath;

  const { queryStringParameters } = event;

  const queryString = new URLSearchParams(
    queryStringParameters as unknown as string,
  ).toString();

  // @ts-expect-error missing aws-lambda types
  const url = `https://${event.headers.Host}${event.rawPath}?${queryString}`;

  try {
    // TODO: use API gateway authorizer instead of this hack
    const apiKey = event.headers['x-api-key'];

    if (apiKey !== process.env.API_KEY) {
      return {
        statusCode: 403,
        headers: {
          'Content-Type': 'application/json',
          'Access-Control-Allow-Origin': '*',
        },
        body: JSON.stringify({ message: 'Forbidden' }, null, 2),
      };
    }
    return await Promise.race([
      routes(path, queryStringParameters, url),
      lambdaTimeout(context),
    ]).then(value => value);

    // const url = `https://${event.headers.Host}${path}?${queryStringParameters}`;
  } catch (e) {
    return {
      statusCode: 500,
      headers: {
        'Content-Type': 'application/json',
        'Access-Control-Allow-Origin': '*',
      },
      body: e,
    };
  }
};
