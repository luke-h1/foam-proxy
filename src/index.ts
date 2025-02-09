import { APIGatewayProxyEvent, Context, Handler } from 'aws-lambda';
import routes from './routes';
import lambdaTimeout from './util/lambdaTimeout';

export const handler: Handler = async (
  event: APIGatewayProxyEvent,
  context: Context,
) => {
  const path = event.path;

  console.info('path is ->', path);

  const { queryStringParameters } = event;

  const queryString = new URLSearchParams(
    queryStringParameters as unknown as string,
  ).toString();

  const url = `https://${event.headers.Host}${event.path}?${queryString}`;

  console.info('url ->', url);

  try {
    // TODO: use API gateway authorizer instead of this hack
    // const apiKey = event.headers['x-api-key'];

    // if (apiKey !== process.env.API_KEY) {
    //   return {
    //     statusCode: 403,
    //     headers: {
    //       'Content-Type': 'application/json',
    //       'Access-Control-Allow-Origin': '*',
    //     },
    //     body: JSON.stringify({ message: 'Forbidden' }, null, 2),
    //   };
    // }
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
      body: e,
    };
  }
};
