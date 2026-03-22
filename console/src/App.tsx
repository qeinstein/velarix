
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import LandingPage from './pages/LandingPage';
import Dashboard from './pages/Dashboard';
import Login from './pages/Login';
import Signup from './pages/Signup';
import Settings from './pages/Settings';
import NeuralGraph from './pages/NeuralGraph';
import Layout from './components/Layout';
import Docs from './pages/Docs';
import Explanations from './pages/Explanations';

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/login" element={<Login />} />
        <Route path="/signup" element={<Signup />} />
        <Route path="/docs" element={<Docs />} />
        
        {/* Protected app routes inside custom layout */}
        <Route element={<Layout />}>
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/graph" element={<NeuralGraph />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/explanations" element={<Explanations />} />
        </Route>
      </Routes>
    </Router>
  );
}

export default App;
