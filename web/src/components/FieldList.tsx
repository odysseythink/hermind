import type { SchemaDescriptor } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';

export interface FieldListProps {
  descriptor: SchemaDescriptor;
  options: Record<string, string>;
  onChange: (field: string, value: string) => void;
}

export default function FieldList({ descriptor, options, onChange }: FieldListProps) {
  return (
    <div>
      {descriptor.fields.map(field => {
        const value = options[field.name] ?? '';
        const onFieldChange = (v: string) => onChange(field.name, v);
        switch (field.kind) {
          case 'int':
            return <NumberInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'bool':
            return <BoolToggle key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'enum':
            return <EnumSelect key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'secret':
            // Stage 4a: secrets render as plain text inputs. Stage 4b
            // swaps in SecretInput with /reveal.
            return <TextInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'string':
          default:
            return <TextInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
        }
      })}
    </div>
  );
}
