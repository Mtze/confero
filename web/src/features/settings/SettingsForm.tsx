import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateMySettings } from '../../api'
import { client } from '../../lib/query'
import { Button } from '../../components/ui/Button'
import { Input } from '../../components/ui/Input'
import type { UserSettings } from '../../api'

const DAY_NAMES = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

const schema = z.object({
  timezone: z.string().min(1, 'Timezone is required'),
  weekly_digest_enabled: z.boolean(),
  weekly_digest_day: z.number().int().min(0).max(6),
  weekly_digest_hour: z.number().int().min(0).max(23),
  weekly_digest_horizon_weeks: z.number().int().min(1).max(52),
})

type FormValues = z.infer<typeof schema>

interface SettingsFormProps {
  settings: UserSettings
}

export function SettingsForm({ settings }: SettingsFormProps) {
  const qc = useQueryClient()
  const [leadDays, setLeadDays] = useState<number[]>([...settings.reminder_lead_days].sort((a, b) => a - b))
  const [newDay, setNewDay] = useState('')

  const { register, handleSubmit, watch, formState: { errors } } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      timezone: settings.timezone,
      weekly_digest_enabled: settings.weekly_digest_enabled,
      weekly_digest_day: settings.weekly_digest_day,
      weekly_digest_hour: settings.weekly_digest_hour,
      weekly_digest_horizon_weeks: settings.weekly_digest_horizon_weeks,
    },
  })

  const digestEnabled = watch('weekly_digest_enabled')

  const mutation = useMutation({
    mutationFn: (data: FormValues) =>
      updateMySettings({
        client,
        body: {
          ...data,
          reminder_lead_days: leadDays,
        },
      }).then(r => r.data!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] })
    },
  })

  const addLeadDay = () => {
    const n = parseInt(newDay, 10)
    if (!isNaN(n) && n > 0 && !leadDays.includes(n)) {
      setLeadDays(prev => [...prev, n].sort((a, b) => a - b))
      setNewDay('')
    }
  }

  const removeLeadDay = (d: number) => {
    setLeadDays(prev => prev.filter(x => x !== d))
  }

  return (
    <form onSubmit={handleSubmit(d => mutation.mutate(d))} className="flex flex-col gap-6">
      <section className="flex flex-col gap-3">
        <h2 className="text-base font-semibold">General</h2>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Timezone (IANA)</label>
          <Input {...register('timezone')} placeholder="Europe/Berlin" error={errors.timezone?.message} />
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-base font-semibold">Deadline reminders</h2>
        <div className="flex flex-col gap-2">
          <p className="text-sm text-gray-600">Days before each deadline to send a reminder:</p>
          <div className="flex flex-wrap gap-2">
            {leadDays.map(d => (
              <span
                key={d}
                className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-3 py-1 text-sm text-blue-800"
              >
                {d}d
                <button
                  type="button"
                  onClick={() => removeLeadDay(d)}
                  className="text-blue-600 hover:text-red-600 ml-1 font-bold"
                  aria-label={`Remove ${d} day reminder`}
                >
                  x
                </button>
              </span>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              type="number"
              value={newDay}
              onChange={e => setNewDay(e.target.value)}
              placeholder="Days"
              className="w-24"
              min={1}
            />
            <Button type="button" variant="secondary" size="sm" onClick={addLeadDay}>Add</Button>
          </div>
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <h2 className="text-base font-semibold">Weekly digest</h2>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" {...register('weekly_digest_enabled')} className="rounded" />
            Enabled
          </label>
        </div>
        {digestEnabled && (
          <div className="grid grid-cols-3 gap-4">
            <div className="flex flex-col gap-1">
              <label className="text-sm font-medium">Day</label>
              <select {...register('weekly_digest_day', { valueAsNumber: true })} className="rounded border border-gray-300 px-3 py-2 text-sm">
                {DAY_NAMES.map((name, i) => (
                  <option key={i} value={i}>{name}</option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-sm font-medium">Hour (0-23)</label>
              <Input type="number" {...register('weekly_digest_hour', { valueAsNumber: true })} min={0} max={23} />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-sm font-medium">Horizon (weeks)</label>
              <Input type="number" {...register('weekly_digest_horizon_weeks', { valueAsNumber: true })} min={1} max={52} />
            </div>
          </div>
        )}
      </section>

      {mutation.isSuccess && (
        <p className="text-sm text-green-600">Settings saved.</p>
      )}
      {mutation.isError && (
        <p className="text-sm text-red-600">Failed to save settings. Please try again.</p>
      )}

      <div className="flex justify-end">
        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending ? 'Saving...' : 'Save settings'}
        </Button>
      </div>
    </form>
  )
}
