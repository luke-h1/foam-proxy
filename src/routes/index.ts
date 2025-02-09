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
    // case 'default-token':
    // case '/api/default-token':
    //   response = await defaultTokenHandler();
    //   statusCode = 200;
    //   break;

    case 'proxy':
    case '/api/proxy':
      statusCode = 302;
      // const searchParams = new URLSearchParams(
      //   queryParams as Record<string, string>,
      // ).toString();

      // const redirectUri = `foam://?${searchParams}`;

      const redirectUri = `foam://?${new URL(requestUrl, 'http://foam/').searchParams}'`;

      headers = {
        ...headers,
        Location: redirectUri,
      };
      response = JSON.stringify({ message: 'redirecting to app' }, null, 2);
      break;

    case 'pending':
    case '/api/pending':
      statusCode = 200;
      headers['Content-Type'] = 'text/html';

      response = `<html>
        <head>
          <title>Redirecting...</title>
        </head>
        <body>
          <h1>Redirecting...</h1>
          <script>
            setTimeout(() => {
              window.location.href = 'foam://?${new URL(requestUrl, 'http://foam/').searchParams}';
            }, 1000);
          </script>
        </body>
        </html>`;
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
