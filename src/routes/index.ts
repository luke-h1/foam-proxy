/* eslint-disable no-case-declarations */
import defaultTokenHandler from '@lambda/handlers/defaultTokenHandler';
import { APIGatewayProxyEventQueryStringParameters } from 'aws-lambda';

const routes = async (
  path: string,
  _queryParams: APIGatewayProxyEventQueryStringParameters | null,
  requestUrl: string,
) => {
  let response: unknown = {
    status: 'OK',
  };
  let statusCode: number;

  let headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET,OPTIONS,POST,PUT,DELETE',
  };

  switch (path) {
    // case 'default-token':
    // case '/api/default-token':
    //   response = await defaultTokenHandler();
    //   statusCode = 200;
    //   break;

    case '/api/proxy':
    case '/proxy':
      statusCode = 302;

      const redirectUri =
        `foam://?` + new URL(requestUrl, 'http://a').searchParams;

      headers = {
        ...headers,
        Location: redirectUri,
      };
      response = JSON.stringify({ message: 'redirecting to app' }, null, 2);
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
