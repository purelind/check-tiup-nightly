import { NextRequest, NextResponse } from 'next/server';

const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:5050';

export async function GET(
  request: NextRequest,
  { params }: { params: { platform: string } }
) {
  const searchParams = request.nextUrl.searchParams;
  const days = searchParams.get('days') || '1';
  const platform = params.platform;

  try {
    const response = await fetch(
      `${API_BASE_URL}/platforms/${platform}/history?days=${days}`
    );
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    return NextResponse.json(
      { error: 'Failed to fetch history data' },
      { status: 500 }
    );
  }
}
