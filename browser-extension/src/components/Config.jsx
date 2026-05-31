import React from 'react';

export default function Config({ status, apiBase, onConnect, onDisconnect }) {
  const [input, setInput] = React.useState('');

  if (status === 'connected') {
    return (
      <div className="p-4">
        <div className="flex items-center gap-2 mb-4">
          <span className="text-green-500 text-xl">✅</span>
          <span className="font-medium">Connected to Hermind</span>
        </div>
        <p className="text-sm text-gray-600 mb-4 break-all">{apiBase}</p>
        <button
          onClick={onDisconnect}
          className="w-full px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600"
        >
          Disconnect
        </button>
      </div>
    );
  }

  return (
    <div className="p-4">
      <h2 className="text-lg font-semibold mb-2">Connect to Hermind</h2>
      <p className="text-sm text-gray-600 mb-4">
        Paste your connection string from the Hermind settings page.
      </p>
      <input
        type="text"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="https://example.com/api|brx-..."
        className="w-full px-3 py-2 border rounded mb-3 text-sm"
      />
      <button
        onClick={() => onConnect(input)}
        disabled={status === 'connecting'}
        className="w-full px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
      >
        {status === 'connecting' ? 'Connecting...' : 'Connect'}
      </button>
      {status === 'error' && (
        <p className="text-red-500 text-sm mt-2">Connection failed. Check your API key.</p>
      )}
    </div>
  );
}