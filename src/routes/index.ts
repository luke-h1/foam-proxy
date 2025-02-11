/* eslint-disable no-case-declarations */
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
      response = `
        <html>
          <head>
            <title>Pending</title>
          </head>
          <body>
            <h1>Your request is pending</h1>
            <p>Please wait while we process your request.</p>
          </body>
        </html>
      `;
    }
      break;

    case '/api/proxy': {
      statusCode = 302;

      const redirectUri =
        `foam://?` + new URL(requestUrl, 'http://a').searchParams;

      console.info('redirectUri', redirectUri);

      headers = {
        ...headers,
        Location: redirectUri,
      };
      response = JSON.stringify({ message: 'redirecting to app' }, null, 2);
    }

      break;

    case '/api/healthcheck': {
      statusCode = 200;

      response = JSON.stringify({
        message: 'OK'
      }, null, 2)
    }

      break;

    default:
      // Match any root path and redirect to the app
      if (path === '' || path === '/') {
        statusCode = 302;
        const redirectUri = 'foam://';
        console.info('redirecting to app:', redirectUri);
        headers = {
          ...headers,
          Location: redirectUri,
        };
        response = '';
      } else {
        console.info('path', path);
        response = JSON.stringify({ message: 'route not found' }, null, 2);
        statusCode = 404;
      }
      break;
  }
  return {
    statusCode,
    headers,
    body: response,
  };
};
export default routes;
