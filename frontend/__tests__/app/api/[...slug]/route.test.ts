/**
 * @jest-environment node
 */

import {GET, resetCache} from '@/app/api/terminology/[...slug]/route'
import { NextRequest } from 'next/server'

// Mock fetch globally
global.fetch = jest.fn()

// Mock environment variables
const mockEnv = {
    TERMINOLOGY_SERVER_BASE_URL: 'https://terminology.example.com',
    TERMINOLOGY_SERVER_USERNAME: 'testuser',
    TERMINOLOGY_SERVER_PASSWORD: 'testpass'
}

Object.assign(process.env, mockEnv)

describe('Terminology API Route', () => {
    beforeEach(() => {
        jest.clearAllMocks()
        jest.clearAllTimers()
        jest.useFakeTimers()
        resetCache()
    })

    afterEach(() => {
        jest.useRealTimers()
    })

    it('returns terminology data when request succeeds', async () => {
        const mockTerminologyData = { concepts: [{ code: 'test', display: 'Test' }] }

        ;(fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ access_token: 'token123', expires_in: 3600 })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockTerminologyData)
            })

        const request = new NextRequest('http://localhost:3000/api/terminology/ValueSet?url=test')

        const response = await GET(request)
        const data = await response.json()

        expect(data).toEqual(mockTerminologyData)
        expect(response.status).toBe(200)
    })

    it('includes query parameters in proxied request', async () => {
        ;(fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ access_token: 'token123', expires_in: 3600 })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({})
            })

        const request = new NextRequest('http://localhost:3000/api/terminology/ValueSet?url=test&version=1.0')

        await GET(request)

        expect(fetch).toHaveBeenLastCalledWith(
            'https://terminology.example.com/ValueSet?url=test&version=1.0',
            expect.objectContaining({
                headers: { Authorization: 'Bearer token123' }
            })
        )
    })

    it('handles nested slug paths correctly', async () => {
        ;(fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ access_token: 'token123', expires_in: 3600 })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({})
            })

        const request = new NextRequest('http://localhost:3000/api/terminology/ValueSet/expand')

        await GET(request)

        expect(fetch).toHaveBeenLastCalledWith(
            'https://terminology.example.com/ValueSet/expand?',
            expect.any(Object)
        )
    })

    it('returns error when terminology server request fails', async () => {
        ;(fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ access_token: 'token123', expires_in: 3600 })
            })
            .mockResolvedValueOnce({
                ok: false,
                statusText: 'Not Found'
            })

        const request = new NextRequest('http://localhost:3000/api/terminology/ValueSet')

        try {
            await GET(request)
        }
        catch (error) {
            expect(error).toEqual(new Error("Failed to proxy ValueSet"))
        }
    })

    it('returns error when token request fails', async () => {
        ;(fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: false,
                statusText: 'Unauthorized'
            })

        const request = new NextRequest('http://localhost:3000/api/terminology/ValueSet')
        try {
            await GET(request)
        } catch (error) {
            expect(error).toEqual(new Error("Failed to get token: Unauthorized"))

        }

    })

    it('reuses cached token when not expired', async () => {
        (fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ access_token: 'token123', expires_in: 3600 })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({})
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({})
            })

        const request1 = new NextRequest('http://localhost:3001/api/terminology/ValueSet')
        const request2 = new NextRequest('http://localhost:3001/api/terminology/CodeSystem')

        await GET(request1)
        await GET(request2)

        expect(fetch).toHaveBeenCalledTimes(4)
    })

    it('returns cached response when available and not expired', async () => {
        const mockData = { concepts: [{ code: 'cached', display: 'Cached' }] }

        ;(fetch as jest.Mock)
            .mockResolvedValueOnce({
                json: () => Promise.resolve({ token_endpoint: 'https://auth.example.com/token' })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ access_token: 'token123', expires_in: 3600 })
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockData)
            })

        const request1 = new NextRequest('http://localhost:3000/api/terminology/ValueSet?url=same1')
        const request2 = new NextRequest('http://localhost:3000/api/terminology/ValueSet?url=same1')

        const response1 = await GET(request1)
        const response2 = await GET(request2)

        const data1 = await response1.json()
        const data2 = await response2.json()

        expect(data1).toEqual(mockData)
        expect(data2).toEqual(mockData)
        expect(fetch).toHaveBeenCalledTimes(3)
    })
})