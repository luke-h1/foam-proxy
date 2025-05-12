import { Context } from 'aws-lambda';
import LambdaError from './lambdaError';

const lambdaTimeout = (context: Context) =>
  new Promise((_res, rej) => {
    setTimeout(() => {
      rej(
        new LambdaError({
          message: 'Lambda Timeout',
          name: 'LambdaTimeout',
          code: 500,
          statusCode: 500,
          action: 'timeout',
        }),
      );
    }, context.getRemainingTimeInMillis() - 1000);
  });
export default lambdaTimeout;
