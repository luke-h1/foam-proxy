/* eslint-disable no-case-declarations */
import defaultTokenHandler from '@lambda/handlers/defaultTokenHandler';
import { Dictionary } from '@lambda/types/util';
import { APIGatewayProxyEventQueryStringParameters } from 'aws-lambda';

const routes = async (
  path: string,
  queryParams: APIGatewayProxyEventQueryStringParameters | null,
) => {
  let response: unknown = {
    status: 'OK',
  };
  let statusCode: number;

  let headers: Dictionary<string> = {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET,OPTIONS,POST,PUT,DELETE',
  };

  switch (path) {
    case 'default-token':
    case '/api/default-token':
      response = await defaultTokenHandler();
      statusCode = 200;
      break;

    case 'proxy':
    case '/api/proxy':
      statusCode = 302;

      const searchParams = new URLSearchParams(
        queryParams as Record<string, string>,
      ).toString();
      const redirectUri = `foam://?${searchParams}`;

      headers = {
        ...headers,
        Location: redirectUri,
      };
      response = JSON.stringify({ message: 'redirecting to app' }, null, 2);

      console.info('redirecting to app:', redirectUri);
      console.info('redirecting to app:', searchParams);
      console.info('redirecting to app:', headers);
      break;

    default:
      response = JSON.stringify({ message: 'route not found' }, null, 2);
      statusCode = 404;
      break;
  }

  return {
    statusCode,
    headers,
    body: response,
  };
};
export default routes;
