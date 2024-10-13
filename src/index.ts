import { APIGatewayProxyEvent, Context, Handler } from 'aws-lambda';
import lambdaTimeout from './util/lambdaTimeout';
import routes from './routes';

export const handler: Handler = async (
  event: APIGatewayProxyEvent,
  context: Context,
) => {
  // TODO: validate that all requests come from the app *only*

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

  try {
    return await Promise.race([routes(path), lambdaTimeout(context)]).then(
      value => value,
    );
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
