import type { SchemaDescriptor } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';
import SecretInput from './fields/SecretInput';

export interface FieldListProps {
  descriptor: SchemaDescriptor;
  options: Record<string, string>;
  originalOptions: Record<string, string>;
  instanceKey: string;
  instanceIsNew: boolean;
  onChange: (field: string, value: string) => void;
}

export default function FieldList({
  descriptor,
  options,
  originalOptions,
  instanceKey,
  instanceIsNew,
  onChange,
}: FieldListProps) {
  return (
    <div>
      {descriptor.fields.map(field => {
        const value = options[field.name] ?? '';
        const originalValue = originalOptions[field.name] ?? '';
        const onFieldChange = (v: string) => onChange(field.name, v);
        switch (field.kind) {
          case 'int':
            return <NumberInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'bool':
            return <BoolToggle key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'enum':
            return <EnumSelect key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'secret':
            return (
              <SecretInput
                key={field.name}
                field={field}
                value={value}
                instanceKey={instanceKey}
                dirty={instanceIsNew || value !== originalValue}
                onChange={onFieldChange}
              />
            );
          case 'string':
          default:
            return <TextInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
        }
      })}
    </div>
  );
}
