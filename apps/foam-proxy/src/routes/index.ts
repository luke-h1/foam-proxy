/* eslint-disable no-console */
/* eslint-disable no-shadow */
/* eslint-disable no-case-declarations */
import * as newrelic from 'newrelic';
import healthHandler from '@lambda/handlers/health';
import pendingHandler from '@lambda/handlers/pending';
import proxyHandler from '@lambda/handlers/proxy';
import tokenHandler from '@lambda/handlers/token';
import versionHandler from '@lambda/handlers/version';
import { APIGatewayProxyEventQueryStringParameters } from 'aws-lambda';

const routes = async (
  path: string,
  _queryParams: APIGatewayProxyEventQueryStringParameters | null,
  requestUrl: string,
) => {
  let response: unknown;
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
      // eslint-disable-next-line @typescript-eslint/restrict-template-expressions
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

    case '/api/token': {
      statusCode = 200;
      response = await tokenHandler();
      break;
    }

    case '/api/healthcheck': {
      statusCode = 200;
      console.info('Health check request received');
      newrelic.incrementMetric('Custom/HealthCheck/Requests');
      newrelic.addCustomAttribute('endpoint', 'healthcheck');
      response = healthHandler();
      break;
    }

    case '/api/version': {
      statusCode = 200;
      console.info('Version request received');
      newrelic.incrementMetric('Custom/Version/Requests');
      newrelic.addCustomAttribute('endpoint', 'version');
      response = versionHandler();
      break;
    }

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
