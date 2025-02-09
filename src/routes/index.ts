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

    case '/api/pending':
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
      break;
    case '/api/proxy':
    case '/proxy':
      statusCode = 302;

      const redirectUri =
        `foam://?` + new URL(requestUrl, 'http://a').searchParams;

      console.info('redirectUri', redirectUri);

      headers = {
        ...headers,
        Location: redirectUri,
      };
      response = JSON.stringify({ message: 'redirecting to app' }, null, 2);
      break;

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
