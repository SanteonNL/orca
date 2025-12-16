'use client'

import "./globals.css";
import { getSupportContactEmail } from '@/app/actions'
import { ErrorWithTitle } from "@/app/utils/error-with-title";
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'

type ErrorPageProps = {
  error: Error & {
    digest?: string
  }
  reset: () => void
}

const ErrorPage = ({ error }: ErrorPageProps) => {
  return <DefaultErrorPage error={error} />
}

export default ErrorPage

export const DefaultErrorPage = ({
  error
}:  {
  canReset?: boolean
  error?: Error
}) => {
  useEffect(() => {
    console.error(error)
  }, [error])

  const { data: supportContact } = useQuery({
    queryKey: ['supportContact'],
    queryFn: async () => getSupportContactEmail() ?? null,
    staleTime: Infinity
  })

  const errorWithTitle = error instanceof ErrorWithTitle ? error : null;

  return (
    <div className="grid h-screen w-full items-center justify-center">
      <div className="flex max-w-sm flex-col gap-5">
        <h2 className="text-3xl font-bold">
          <span className="text-[#0096a1]">Oeps!</span>{' '}
          {errorWithTitle ? errorWithTitle.title : <>Er is iets misgegaan</>}
        </h2>
        <p className="whitespace-nowrap">
          {errorWithTitle ? (
            errorWithTitle.userMessage
          ) : (
            <>
              We konden dit scherm niet laden. Probeer het alsjeblieft opnieuw.
            </>
          )}
        </p>
        <p>
          Blijft dit probleem zich voordoen? Neem dan contact op
          {supportContact ? (
            <>
              {' '}
              via <span className="font-bold">{supportContact}</span>
            </>
          ) : (
            '.'
          )}
        </p>
      </div>
    </div>
  )
}
