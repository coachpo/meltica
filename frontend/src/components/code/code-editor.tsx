'use client';

import {
  forwardRef,
  useMemo,
  type AriaAttributes,
} from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { githubDark, githubLight } from '@uiw/codemirror-themes-all';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';
import { EditorView, keymap } from '@codemirror/view';

import { useTheme } from '@/components/ui/theme-provider';
import { cn } from '@/lib/utils';

export type CodeEditorLanguage = 'javascript' | 'json' | 'text';

export interface CodeEditorProps extends AriaAttributes {
  id?: string;
  value: string;
  onChange?: (next: string) => void;
  language?: CodeEditorLanguage;
  readOnly?: boolean;
  className?: string;
  editorClassName?: string;
  height?: string | number;
  onSubmitShortcut?: () => void;
  wrapLines?: boolean;
  allowHorizontalScroll?: boolean;
  autoFocus?: boolean;
  placeholder?: string;
}

export const CodeEditor = forwardRef<HTMLDivElement, CodeEditorProps>(function CodeEditor(
  props,
  ref,
) {
  const {
    value,
    onChange,
    language = 'javascript',
    readOnly = false,
    className,
    editorClassName,
    height = '18rem',
    onSubmitShortcut,
    wrapLines,
    allowHorizontalScroll,
    autoFocus,
    id,
    placeholder,
    ...ariaProps
  } = props;
  const { resolvedTheme } = useTheme();

  const shouldWrap = wrapLines ?? !allowHorizontalScroll;

  const extensions = useMemo(() => {
    const list = [];
    if (language === 'json') {
      list.push(json());
    } else if (language === 'javascript') {
      list.push(javascript({ jsx: true, typescript: true }));
    }
    if (shouldWrap) {
      list.push(EditorView.lineWrapping);
    }
    if (onSubmitShortcut) {
      list.push(
        keymap.of([
          {
            key: 'Mod-Enter',
            run: () => {
              onSubmitShortcut();
              return true;
            },
          },
        ]),
      );
    }
    return list;
  }, [language, onSubmitShortcut, shouldWrap]);

  const resolvedHeight = typeof height === 'number' ? `${height}px` : height;

  return (
    <div ref={ref} className={cn('rounded-md border bg-background', className)}>
      <CodeMirror
        id={id}
        value={value}
        onChange={(next) => onChange?.(next)}
        height={resolvedHeight}
        theme={resolvedTheme === 'dark' ? githubDark : githubLight}
        extensions={extensions}
        editable={!readOnly}
        className={editorClassName}
        basicSetup={{
          lineNumbers: true,
          foldGutter: true,
          highlightActiveLine: true,
          highlightActiveLineGutter: true,
        }}
        autoFocus={autoFocus}
        placeholder={placeholder}
        {...ariaProps}
      />
    </div>
  );
});
