'use client';

import { CodeEditor, type CodeEditorProps } from './code-editor';

type ViewerOmittedProps = 'onChange' | 'readOnly' | 'onSubmitShortcut';

export type CodeViewerProps = Omit<CodeEditorProps, ViewerOmittedProps>;

export function CodeViewer({ value, ...props }: CodeViewerProps) {
  return (
    <CodeEditor
      value={value}
      readOnly
      wrapLines={props.wrapLines}
      allowHorizontalScroll={props.allowHorizontalScroll}
      {...props}
    />
  );
}
