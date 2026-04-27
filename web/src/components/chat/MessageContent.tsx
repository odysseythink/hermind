import { memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import CodeBlock from './markdown/CodeBlock';

type Props = { content: string };

// Above this size, ReactMarkdown + remark-math + rehype-katex blow up
// the main thread (300KB log pastes were freezing the page for 30+s
// because remark-math walks the whole text scanning for `$` delimiters
// and rehype-katex parses every false positive). Plain <pre> is fine
// for anything that big — it's almost always pasted output, not prose.
const MARKDOWN_SIZE_LIMIT = 32 * 1024;

function MessageContentImpl({ content }: Props) {
  if (content.length > MARKDOWN_SIZE_LIMIT) {
    return <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{content}</pre>;
  }
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath]}
      rehypePlugins={[[rehypeKatex, { strict: false, output: 'mathml' }]]}
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

export default memo(MessageContentImpl);
