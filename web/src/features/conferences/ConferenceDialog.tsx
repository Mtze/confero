import * as Dialog from '@radix-ui/react-dialog'
import type { Conference } from '../../api'
import { ConferenceForm } from './ConferenceForm'

interface ConferenceDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  conference?: Conference
  onSuccess?: (conf: Conference) => void
}

export function ConferenceDialog({ open, onOpenChange, conference, onSuccess }: ConferenceDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 z-40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-full max-w-2xl -translate-x-1/2 -translate-y-1/2 rounded-lg bg-white p-6 shadow-xl">
          <Dialog.Title className="text-lg font-semibold mb-4">
            {conference ? 'Edit conference' : 'New conference'}
          </Dialog.Title>
          <ConferenceForm
            conference={conference}
            onSuccess={(conf) => {
              onOpenChange(false)
              onSuccess?.(conf)
            }}
            onCancel={() => onOpenChange(false)}
          />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
