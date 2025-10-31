import React, { useState, useEffect, useCallback } from 'react';
import './index.css';

const WEBSOCKET_URL = process.env.NODE_ENV === 'production' 
  ? `wss://${window.location.host}/ws`
  : 'ws://localhost:8080/ws';

function App() {
  const [ws, setWs] = useState(null);
  const [connected, setConnected] = useState(false);
  const [username, setUsername] = useState('');
  const [gameId, setGameId] = useState('');
  const [game, setGame] = useState(null);
  const [player, setPlayer] = useState(null);
  const [isWaiting, setIsWaiting] = useState(false);
  const [error, setError] = useState('');
  const [leaderboard, setLeaderboard] = useState([]);
  const [stats, setStats] = useState(null);
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem('darkMode');
    return saved ? JSON.parse(saved) : false;
  });

  const connectWebSocket = useCallback(() => {
    const websocket = new WebSocket(WEBSOCKET_URL);
    
    websocket.onopen = () => {
      console.log('WebSocket connected');
      setConnected(true);
      setError('');
    };

    websocket.onclose = () => {
      console.log('WebSocket disconnected');
      setConnected(false);
      setTimeout(connectWebSocket, 3000); // Reconnect after 3 seconds
    };

    websocket.onerror = (error) => {
      console.error('WebSocket error:', error);
      setError('Connection error. Retrying...');
    };

    websocket.onmessage = (event) => {
      const message = JSON.parse(event.data);
      handleWebSocketMessage(message);
    };

    setWs(websocket);
  }, []);

  useEffect(() => {
    connectWebSocket();
    fetchLeaderboard();
    fetchStats();

    return () => {
      if (ws) {
        ws.close();
      }
    };
  }, [connectWebSocket]);

  useEffect(() => {
    localStorage.setItem('darkMode', JSON.stringify(darkMode));
    document.body.className = darkMode ? 'dark-mode' : 'light-mode';
  }, [darkMode]);

  const toggleDarkMode = () => {
    setDarkMode(!darkMode);
  };

  const handleWebSocketMessage = (message) => {
    console.log('Received message:', message);
    
    switch (message.type) {
      case 'game_joined':
        setGame(message.data.game);
        setPlayer(message.data.player);
        setIsWaiting(message.data.isWaiting);
        setError('');
        break;
        
      case 'game_started':
        setGame(message.data);
        setIsWaiting(false);
        break;
        
      case 'game_updated':
        setGame(message.data);
        setIsWaiting(message.data.status === 'waiting');
        break;
        
      case 'move_made':
        setGame(message.data.game);
        break;
        
      case 'game_reconnected':
        setGame(message.data);
        setIsWaiting(false);
        break;
        
      case 'error':
        setError(message.data.message);
        break;
        
      default:
        console.log('Unknown message type:', message.type);
    }
  };

  const fetchLeaderboard = async () => {
    try {
      const response = await fetch('/api/leaderboard');
      if (response.ok) {
        const data = await response.json();
        setLeaderboard(data);
      }
    } catch (error) {
      console.error('Failed to fetch leaderboard:', error);
    }
  };

  const fetchStats = async () => {
    try {
      const response = await fetch('/api/stats');
      if (response.ok) {
        const data = await response.json();
        setStats(data);
      }
    } catch (error) {
      console.error('Failed to fetch stats:', error);
    }
  };

  const joinGame = () => {
    if (!username.trim()) {
      setError('Please enter a username');
      return;
    }

    if (ws && connected) {
      const data = { username: username.trim() };
      if (gameId.trim()) {
        data.gameId = gameId.trim();
      }
      
      ws.send(JSON.stringify({
        type: 'join_game',
        data: data
      }));
    } else {
      setError('Not connected to server');
    }
  };

  const makeMove = (column) => {
    if (!game || game.status !== 'playing') return;
    
    const playerNum = player.username === game.player1.username ? 1 : 2;
    if (game.currentTurn !== playerNum) return;

    if (ws && connected) {
      ws.send(JSON.stringify({
        type: 'make_move',
        data: { column }
      }));
    }
  };

  const resetGame = () => {
    setGame(null);
    setPlayer(null);
    setIsWaiting(false);
    setError('');
    setGameId('');
    fetchLeaderboard();
    fetchStats();
  };

  const renderBoard = () => {
    if (!game) return null;

    return (
      <div className="board">
        {game.board.map((row, rowIndex) =>
          row.map((cell, colIndex) => (
            <div
              key={`${rowIndex}-${colIndex}`}
              className={`cell ${
                cell === 0 ? 'empty' : 
                cell === 1 ? 'player1' : 'player2'
              }`}
              onClick={() => makeMove(colIndex)}
            >
              {cell === 1 && '●'}
              {cell === 2 && '●'}
            </div>
          ))
        )}
      </div>
    );
  };

  const getGameStatus = () => {
    if (!game) return '';
    
    if (game.status === 'waiting') {
      return isWaiting ? 'Waiting for opponent...' : 'Game found! Starting...';
    }
    
    if (game.status === 'playing') {
      const currentPlayer = game.currentTurn === 1 ? game.player1 : game.player2;
      const isMyTurn = player && currentPlayer.username === player.username;
      return isMyTurn ? "Your turn!" : `${currentPlayer.username}'s turn`;
    }
    
    if (game.status === 'finished') {
      if (game.winner === 0) {
        return "It's a draw!";
      }
      const winner = game.winner === 1 ? game.player1 : game.player2;
      const isWinner = player && winner.username === player.username;
      return isWinner ? "You won!" : `${winner.username} won!`;
    }
    
    return '';
  };

  const getStatusClass = () => {
    if (!game) return '';
    
    if (game.status === 'waiting') return 'waiting';
    if (game.status === 'playing') return 'playing';
    if (game.status === 'finished') return 'finished';
    
    return '';
  };

  if (!game) {
    return (
      <div className={`app ${darkMode ? 'dark-mode' : 'light-mode'}`}>
        <div className={`connection-status ${connected ? 'connected' : 'disconnected'}`}>
          {connected ? '● Connected' : '● Disconnected'}
        </div>
        
        <button className="theme-toggle" onClick={toggleDarkMode}>
          {darkMode ? 'Light Mode' : 'Dark Mode'}
        </button>
        
        <div className="header">
          <h1>4-in-a-Row</h1>
          <p>Real-time multiplayer Connect Four with intelligent bot</p>
        </div>

        <div className="login-form">
          <h2 style={{ marginBottom: '30px' }}>Join Game</h2>
          <input
            type="text"
            placeholder="Enter your username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            onKeyPress={(e) => e.key === 'Enter' && joinGame()}
            maxLength={20}
          />
          <input
            type="text"
            placeholder="Game ID (optional - for specific game)"
            value={gameId}
            onChange={(e) => setGameId(e.target.value)}
            onKeyPress={(e) => e.key === 'Enter' && joinGame()}
            maxLength={36}
            style={{ marginTop: '10px' }}
          />
          <button 
            onClick={joinGame} 
            disabled={!connected || !username.trim()}
          >
            {connected ? (gameId ? 'Join Specific Game' : 'Start Playing') : 'Connecting...'}
          </button>
          
          {error && <div className="error">{error}</div>}
        </div>

        <div className="game-container">
          <div className="leaderboard">
            <h3>Leaderboard</h3>
            {leaderboard.length > 0 ? (
              leaderboard.map((entry, index) => {
                let timeDisplay = "No wins yet";
                if (entry.bestTime && entry.bestTime > 0) {
                  const minutes = Math.floor(entry.bestTime / 60);
                  const seconds = Math.floor(entry.bestTime % 60);
                  timeDisplay = `${minutes}:${seconds.toString().padStart(2, '0')}`;
                }
                return (
                  <div key={entry.username} className="leaderboard-entry">
                    <span>#{index + 1} {entry.username}</span>
                    <span>{timeDisplay}</span>
                  </div>
                );
              })
            ) : (
              <p>No games played yet</p>
            )}
          </div>

          {stats && (
            <div className="stats">
              <h4>Game Stats</h4>
              <p>Total Games: {stats.totalGames}</p>
              <p>Human vs Human: {stats.humanGames}</p>
              <p>Human vs Bot: {stats.botGames}</p>
              <p>Active Games: {stats.activeGames}</p>
              {stats.avgDuration && (
                <p>Avg Duration: {Math.round(stats.avgDuration)}s</p>
              )}
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className={`app ${darkMode ? 'dark-mode' : 'light-mode'}`}>
      <div className={`connection-status ${connected ? 'connected' : 'disconnected'}`}>
        {connected ? '● Connected' : '● Disconnected'}
      </div>
      
      <button className="theme-toggle" onClick={toggleDarkMode}>
        {darkMode ? 'Light Mode' : 'Dark Mode'}
      </button>
      
      <div className="header">
        <h1>4-in-a-Row</h1>
        <div className="game-id-display">
          <strong>Game ID:</strong> {game.id}
          <br />
          <small>
            Share this ID with friends to let them join your game!
          </small>
        </div>
      </div>

      <div className="game-container">
        <div className="game-board">
          {renderBoard()}
          <button onClick={resetGame} style={{
            width: '100%',
            padding: '10px',
            background: '#6c757d',
            color: 'white',
            border: 'none',
            borderRadius: '8px',
            cursor: 'pointer'
          }}>
            New Game
          </button>
        </div>

        <div className="game-info">
          <h3>Game Info</h3>
          
          <div className={`player-info ${game.currentTurn === 1 ? 'active' : ''}`}>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <div className="player-color player1-color"></div>
              <span>{game.player1.username}</span>
              {player && player.username === game.player1.username && ' (You)'}
            </div>
            <span>Player 1</span>
          </div>

          <div className={`player-info ${game.currentTurn === 2 ? 'active' : ''}`}>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <div className="player-color player2-color"></div>
              <span>{game.player2 ? game.player2.username : 'Waiting...'}</span>
              {player && game.player2 && player.username === game.player2.username && ' (You)'}
              {game.player2 && game.player2.isBot && ' (Bot)'}
            </div>
            <span>Player 2</span>
          </div>

          <div className={`status ${getStatusClass()}`}>
            {getGameStatus()}
          </div>

          {error && <div className="error">{error}</div>}
        </div>

        <div className="leaderboard">
          <h3>Leaderboard</h3>
          {leaderboard.length > 0 ? (
            leaderboard.map((entry, index) => {
              let timeDisplay = "No wins yet";
              if (entry.bestTime && entry.bestTime > 0) {
                const minutes = Math.floor(entry.bestTime / 60);
                const seconds = Math.floor(entry.bestTime % 60);
                timeDisplay = `${minutes}:${seconds.toString().padStart(2, '0')}`;
              }
              return (
                <div key={entry.username} className="leaderboard-entry">
                  <span>#{index + 1} {entry.username}</span>
                  <span>{timeDisplay}</span>
                </div>
              );
            })
          ) : (
            <p>No games played yet</p>
          )}
        </div>
      </div>
    </div>
  );
}

export default App;