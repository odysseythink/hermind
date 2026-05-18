import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ModelPicker from './ModelPicker';

describe('ModelPicker', () => {
  it('renders model name', () => {
    render(<ModelPicker modelName="gpt-4" />);
    expect(screen.getByText('gpt-4')).toBeInTheDocument();
  });

  it('renders em-dash when no model name', () => {
    render(<ModelPicker modelName="" />);
    expect(screen.getByText('—')).toBeInTheDocument();
  });
});
