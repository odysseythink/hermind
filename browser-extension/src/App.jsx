import React from 'react';
import Config from './components/Config';
import { useApiConnection } from './hooks/useApiConnection';

function App() {
  const { status, apiBase, workspaces, logoUrl, connect, disconnect } = useApiConnection();

  return (
    <div className="bg-white">
      <div className="flex items-center gap-2 p-4 border-b">
        {logoUrl && <img src={logoUrl} alt="logo" className="w-6 h-6" />}
        <h1 className="text-lg font-bold">Hermind</h1>
      </div>
      <Config status={status} apiBase={apiBase} onConnect={connect} onDisconnect={disconnect} />
      {status === 'connected' && workspaces.length > 0 && (
        <div className="px-4 pb-4">
          <p className="text-sm text-gray-600">{workspaces.length} workspace(s) available</p>
        </div>
      )}
    </div>
  );
}

export default App;