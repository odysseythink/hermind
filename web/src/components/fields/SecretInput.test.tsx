import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import SecretInput from './SecretInput';
import type { ConfigField } from '../../api/schemas';

const secretField: ConfigField = {
  name: 'postgres_url',
  label: 'Postgres URL',
  kind: 'secret',
};

describe('SecretInput disableReveal', () => {
  it('disables the Show button when disableReveal is set', () => {
    render(
      <SecretInput
        field={secretField}
        value=""
        disableReveal
        onChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).toBeDisabled();
  });

  it('leaves the Show button enabled when disableReveal is not set', () => {
    render(
      <SecretInput
        field={secretField}
        value=""
        onChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).not.toBeDisabled();
  });
});
