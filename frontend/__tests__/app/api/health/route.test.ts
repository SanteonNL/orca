/**
 * @jest-environment node
 */

import { GET } from '@/app/api/health/route'
import { NextRequest } from 'next/server'

describe('Health API Route', () => {
    it('returns status up when called', async () => {
        const request = new NextRequest('http://localhost:3000/api/health')

        const response = await GET(request)
        const data = await response.json()
        expect(response.headers.get('content-type')).toContain('application/json')
        expect(response.status).toBe(200)
        expect(data).toEqual({ status: 'up' })
    })
})