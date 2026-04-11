import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Settings } from 'lucide-react';
import { usePostSetupMutation } from '@/app/api';

export default function Setup() {
  const navigate = useNavigate();
  const [postSetup, { isLoading }] = usePostSetupMutation();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }
    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    try {
      await postSetup({ username, password }).unwrap();
      navigate('/login', { replace: true });
    } catch {
      setError('Setup failed. Please try again.');
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="card max-w-sm w-full bg-base-200 shadow-xl">
        <div className="card-body items-center text-center">
          <Settings className="w-10 h-10 text-primary mb-2" />
          <h2 className="card-title">Initial Setup</h2>
          <p className="text-sm opacity-60">Step 1 of 1</p>
          <form onSubmit={handleSubmit} className="w-full space-y-3 mt-4">
            <input
              type="text"
              placeholder="Admin username"
              className="input input-bordered w-full"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
            <input
              type="password"
              placeholder="Password"
              className="input input-bordered w-full"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              minLength={8}
              required
            />
            <input
              type="password"
              placeholder="Confirm password"
              className="input input-bordered w-full"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              autoComplete="new-password"
              minLength={8}
              required
            />
            {error && <p className="text-error text-sm">{error}</p>}
            <button
              type="submit"
              className="btn btn-primary btn-block"
              disabled={isLoading}
            >
              {isLoading ? <span className="loading loading-spinner loading-sm" /> : 'Create Admin'}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
