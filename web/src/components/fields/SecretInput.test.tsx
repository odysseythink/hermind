import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import SecretInput from './SecretInput';
import type { SchemaField } from '../../api/schemas';

const secretField: SchemaField = {
  name: 'postgres_url',
  label: 'Postgres URL',
  kind: 'secret',
};

describe('SecretInput disableReveal', () => {
  it('disables the Show button and explains why', () => {
    render(
      <SecretInput
        field={secretField}
        value=""
        instanceKey=""
        dirty={false}
        disableReveal
        onChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute('title', 'Reveal not supported for this field (stage 2)');
  });

  it('leaves the Show button enabled when disableReveal is not set', () => {
    render(
      <SecretInput
        field={secretField}
        value=""
        instanceKey="tg_main"
        dirty={false}
        onChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).not.toBeDisabled();
  });
});
