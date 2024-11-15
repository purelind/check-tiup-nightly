import { NextRequest, NextResponse } from 'next/server';

const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:5050';

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ platform: string }> }
) {
  const platform = (await params).platform;
  const searchParams = request.nextUrl.searchParams;
  const days = searchParams.get('days') || '1';

  try {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/platforms/${platform}/results?days=${days}`
    );

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('History API error:', error);
    return NextResponse.json(
      { error: 'Failed to fetch history data' },
      { status: 500 }
    );
  }
}
