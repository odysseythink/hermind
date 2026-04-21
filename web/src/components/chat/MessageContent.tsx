import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import CodeBlock from './markdown/CodeBlock';

type Props = { content: string };

export default function MessageContent({ content }: Props) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath]}
      rehypePlugins={[rehypeKatex]}
      components={{
        code({ className, children, ...props }) {
          const match = /language-(\w+)/.exec(className || '');
          const isBlock = !!match;
          if (!isBlock) {
            return (
              <code className={className} {...props}>
                {children}
              </code>
            );
          }
          const lang = match?.[1] ?? 'text';
          const text = String(children).replace(/\n$/, '');
          return <CodeBlock language={lang} code={text} />;
        },
      }}
    >
      {content}
    </ReactMarkdown>
  );
}
