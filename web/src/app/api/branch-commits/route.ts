import { NextRequest, NextResponse } from 'next/server';

const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:5050';

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const branch = searchParams.get('branch') || 'master';

  try {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/branch-commits?branch=${branch}`
    );

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('Branch commits API error:', error);
    return NextResponse.json(
      { error: 'Failed to fetch branch commits data' },
      { status: 500 }
    );
  }
}
