import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createConference, updateConference } from '../../api'
import { client } from '../../lib/query'
import { Button } from '../../components/ui/Button'
import { Input } from '../../components/ui/Input'
import type { Conference } from '../../api'

const schema = z.object({
  name: z.string().min(1, 'Name is required'),
  acronym: z.string().min(1, 'Acronym is required'),
  year: z.number().int().min(2000).max(2100),
  location: z.string().min(1, 'Location is required'),
  website_url: z.string().url().nullable().optional().or(z.literal('')),
  cfp_url: z.string().url().nullable().optional().or(z.literal('')),
  primary_deadline: z.string().nullable().optional(),
  abstract_deadline: z.string().nullable().optional(),
  core_rank: z.enum(['A*', 'A', 'B', 'C', 'Unranked']).nullable().optional(),
  notes: z.string().nullable().optional(),
})

type FormValues = z.infer<typeof schema>

interface ConferenceFormProps {
  conference?: Conference
  onSuccess: (conf: Conference) => void
  onCancel: () => void
}

export function ConferenceForm({ conference, onSuccess, onCancel }: ConferenceFormProps) {
  const qc = useQueryClient()
  const isEdit = !!conference

  const { register, handleSubmit, formState: { errors } } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: conference
      ? {
          name: conference.name,
          acronym: conference.acronym,
          year: conference.year,
          location: conference.location,
          website_url: conference.website_url ?? '',
          cfp_url: conference.cfp_url ?? '',
          primary_deadline: conference.primary_deadline ?? '',
          abstract_deadline: conference.abstract_deadline ?? '',
          core_rank: conference.core_rank ?? undefined,
          notes: conference.notes ?? '',
        }
      : { year: new Date().getFullYear() },
  })

  const mutation = useMutation({
    mutationFn: (data: FormValues) => {
      const body = {
        ...data,
        website_url: data.website_url || null,
        cfp_url: data.cfp_url || null,
        primary_deadline: data.primary_deadline || null,
        abstract_deadline: data.abstract_deadline || null,
        core_rank: data.core_rank || undefined,
        notes: data.notes || null,
      }
      if (isEdit) {
        return updateConference({ client, path: { id: conference.id }, body }).then(r => r.data!)
      }
      return createConference({ client, body }).then(r => r.data!)
    },
    onSuccess: (conf) => {
      qc.invalidateQueries({ queryKey: ['conferences'] })
      onSuccess(conf)
    },
  })

  return (
    <form onSubmit={handleSubmit(d => mutation.mutate(d))} className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Name *</label>
          <Input {...register('name')} error={errors.name?.message} />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Acronym *</label>
          <Input {...register('acronym')} error={errors.acronym?.message} />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Year *</label>
          <Input type="number" {...register('year', { valueAsNumber: true })} error={errors.year?.message} />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Location *</label>
          <Input {...register('location')} error={errors.location?.message} />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Website URL</label>
          <Input {...register('website_url')} placeholder="https://" />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">CFP URL</label>
          <Input {...register('cfp_url')} placeholder="https://" />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Primary Deadline</label>
          <Input type="datetime-local" {...register('primary_deadline')} />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">Abstract Deadline</label>
          <Input type="datetime-local" {...register('abstract_deadline')} />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-sm font-medium">CORE Rank</label>
          <select {...register('core_rank')} className="rounded border border-gray-300 px-3 py-2 text-sm">
            <option value="">-</option>
            <option value="A*">A*</option>
            <option value="A">A</option>
            <option value="B">B</option>
            <option value="C">C</option>
            <option value="Unranked">Unranked</option>
          </select>
        </div>
      </div>
      <div className="flex flex-col gap-1">
        <label className="text-sm font-medium">Notes</label>
        <textarea {...register('notes')} rows={3} className="rounded border border-gray-300 px-3 py-2 text-sm" />
      </div>
      {mutation.error && (
        <p className="text-sm text-red-600">Failed to save conference. Please try again.</p>
      )}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending ? 'Saving...' : isEdit ? 'Save changes' : 'Create'}
        </Button>
      </div>
    </form>
  )
}
