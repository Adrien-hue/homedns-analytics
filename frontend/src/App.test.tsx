import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import App from './App';

describe('App', () => {
  it('renders the heading', () => {
    render(<App />);

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: /get started/i,
      }),
    ).toBeInTheDocument();
  });

  it('renders the counter button', () => {
    render(<App />);

    expect(
      screen.getByRole('button', {
        name: /count is 0/i,
      }),
    ).toBeInTheDocument();
  });

  it('renders the documentation section', () => {
    render(<App />);

    expect(
      screen.getByRole('heading', {
        name: /documentation/i,
      }),
    ).toBeInTheDocument();
  });

  it('renders the social section', () => {
    render(<App />);

    expect(
      screen.getByRole('heading', {
        name: /connect with us/i,
      }),
    ).toBeInTheDocument();
  });
});
