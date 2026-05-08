import { useEffect, useState, useCallback } from 'react';
import styles from './DefaultModelEditor.module.css';
import fieldStyles from '../../fields/fields.module.css';

export interface DefaultModelEditorProps {
  /** Current model value, e.g. "qwen_main/glm-5". */
  value: string;
  /** Original value for dirty detection (optional). */
  originalValue?: string;
  /** Config.providers object — keys are provider instance keys. */
  providers: Record<string, Record<string, unknown>>;
  /** Already-fetched model lists per provider key. */
  providerModels: Record<string, string[]>;
  /** Fetch models for a given provider. Should update providerModels in parent state. */
  onFetchModels: (instanceKey: string) => Promise<{ models: string[] }>;
  /** Called with the new "provider/model" string when either dropdown changes. */
  onChange: (value: string) => void;
}

type FetchStatus = 'idle' | 'loading' | 'ok' | 'err';

function parseModelValue(v: string): { provider: string; model: string } {
  const slashIdx = v.indexOf('/');
  if (slashIdx < 0) return { provider: v, model: '' };
  return { provider: v.slice(0, slashIdx), model: v.slice(slashIdx + 1) };
}

export default function DefaultModelEditor({
  value,
  providers,
  providerModels,
  onFetchModels,
  onChange,
}: DefaultModelEditorProps) {
  const { provider: selectedProvider, model: selectedModel } = parseModelValue(value);
  const providerIds = Object.keys(providers).sort();
  const models = selectedProvider ? (providerModels[selectedProvider] ?? []) : [];
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('idle');
  const [fetchError, setFetchError] = useState<string>('');
  const doFetch = useCallback(
    async (key: string) => {
      if (!key) return;
      setFetchStatus('loading');
      setFetchError('');
      try {
        await onFetchModels(key);
        setFetchStatus('ok');
      } catch (err) {
        setFetchStatus('err');
        setFetchError(err instanceof Error ? err.message : String(err));
      }
    },
    [onFetchModels]
  );

  // Auto-fetch models when the selected provider changes and we don't have them cached.
  useEffect(() => {
    if (selectedProvider && !providerModels[selectedProvider] && fetchStatus !== 'loading') {
      doFetch(selectedProvider);
    }
  }, [selectedProvider]); // eslint-disable-line react-hooks/exhaustive-deps

  function handleProviderChange(nextProvider: string) {
    // When provider changes, keep the model empty until user picks one from the new list.
    onChange(nextProvider ? `${nextProvider}/` : '');
    if (nextProvider && !providerModels[nextProvider]) {
      doFetch(nextProvider);
    }
  }

  function handleModelChange(nextModel: string) {
    onChange(selectedProvider ? `${selectedProvider}/${nextModel}` : nextModel);
  }

  return (
    <section className={styles.editor} aria-label="Default model">
      <h2 className={styles.title}>Default model</h2>
      <p className={styles.summary}>Model used when a request does not pin one explicitly.</p>

      <label className={fieldStyles.row}>
        <span className={fieldStyles.label}>
          Provider<span className={fieldStyles.required}>*</span>
        </span>
        <select
          className={fieldStyles.select}
          value={selectedProvider}
          onChange={(e) => handleProviderChange(e.currentTarget.value)}
          aria-label="Provider"
        >
          <option value="">— Select provider —</option>
          {providerIds.map((id) => (
            <option key={id} value={id}>
              {id}
            </option>
          ))}
        </select>
      </label>

      <label className={fieldStyles.row}>
        <span className={fieldStyles.label}>
          Model<span className={fieldStyles.required}>*</span>
        </span>
        {fetchStatus === 'loading' ? (
          <span className={styles.loading}>Loading models…</span>
        ) : fetchStatus === 'err' ? (
          <>
            <input
              type="text"
              className={fieldStyles.input}
              value={selectedModel}
              onChange={(e) => handleModelChange(e.currentTarget.value)}
              placeholder="Provider-qualified model id"
              aria-label="Model"
            />
            <span className={styles.chipErr}>{fetchError}</span>
          </>
        ) : models.length > 0 ? (
          <select
            className={fieldStyles.select}
            value={selectedModel}
            onChange={(e) => handleModelChange(e.currentTarget.value)}
            aria-label="Model"
          >
            <option value="">— Select model —</option>
            {models.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        ) : (
          <input
            type="text"
            className={fieldStyles.input}
            value={selectedModel}
            onChange={(e) => handleModelChange(e.currentTarget.value)}
            placeholder={
              selectedProvider
                ? 'Click "Fetch models" in the provider settings to populate this list'
                : 'Select a provider first'
            }
            aria-label="Model"
          />
        )}
      </label>
    </section>
  );
}
