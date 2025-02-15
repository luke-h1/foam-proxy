/* eslint-disable no-console */
/* eslint-disable no-shadow */
/* eslint-disable no-case-declarations */
import healthHandler from '@lambda/handlers/health';
import pendingHandler from '@lambda/handlers/pending';
import proxyHandler from '@lambda/handlers/proxy';
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
    case '/api/pending': {
      statusCode = 200;
      headers['Content-Type'] = 'text/html';
      response = pendingHandler();
      break;
    }

    case '/api/proxy': {
      statusCode = 302;
      const redirectUri = `foam://?${new URL(requestUrl, 'http://a').searchParams}`;

      // eslint-disable-next-line no-console
      console.info('redirectUri', redirectUri);

      headers = {
        ...headers,
        Location: redirectUri,
      };
      response = proxyHandler();

      break;
    }

    case '/api/healthcheck': {
      statusCode = 200;
      response = healthHandler();
      break;
    }

    case '/api/version': {
      statusCode = 200;
      response = versionHandler();
      break;
    }

    default:
      console.info('path', path);
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
