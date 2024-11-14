import { NextResponse } from 'next/server';
 
export async function GET() {
  try {
    const response = await fetch('http://localhost:5050/results');
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    return NextResponse.json(
      { error: 'Failed to fetch data' },
      { status: 500 }
    );
  }
}