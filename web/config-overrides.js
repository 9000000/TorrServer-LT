const webpack = require('webpack');

module.exports = function override(config) {
  const fallback = config.resolve.fallback || {};
  Object.assign(fallback, {
    "crypto": require.resolve("crypto-browserify"),
    "stream": require.resolve("stream-browserify"),
    "path": require.resolve("path-browserify"),
    "os": require.resolve("os-browserify/browser"),
    "querystring": require.resolve("querystring-es3"),
    "fs": false,
    "net": false,
    "http": false,
    "https": false,
    "url": false,
    "zlib": false,
    "child_process": false,
  });
  config.resolve.fallback = fallback;
  config.plugins = (config.plugins || []).concat([
    new webpack.ProvidePlugin({
      process: 'process/browser.js',
      Buffer: ['buffer', 'Buffer']
    })
  ]);
  // Workaround for some cases where mjs/js files are not correctly resolved
  config.module.rules.push({
    test: /\.m?js/,
    resolve: {
        fullySpecified: false
    }
  });
  return config;
}
