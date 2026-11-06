import { useConfirm } from 'primevue/useconfirm'

interface ConfirmOptions {
  header: string
  /** What happens, in words: what is removed and what stays. */
  message: string
  /** The button that goes ahead, named for what it does ("Remove the copy"). */
  accept: string
  /** The button that keeps things as they are. */
  reject?: string
  /** A removal: the accept button is drawn as danger. */
  danger?: boolean
}

/** The one way to ask before an irreversible action. The action runs only on yes. */
export function useConfirmAction() {
  const confirm = useConfirm()
  return (o: ConfirmOptions, action: () => Promise<unknown>) => {
    confirm.require({
      header: o.header,
      message: o.message,
      acceptProps: { label: o.accept, severity: o.danger ? 'danger' : undefined },
      rejectProps: { label: o.reject ?? 'Keep it', severity: 'secondary' },
      accept: () => void action(),
    })
  }
}
