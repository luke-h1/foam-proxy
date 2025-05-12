const proxyHandler = () => {
  return JSON.stringify({ message: 'redirecting to app' }, null, 2);
};
export default proxyHandler;
