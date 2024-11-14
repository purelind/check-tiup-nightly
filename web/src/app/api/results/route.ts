import { NextResponse } from 'next/server';

const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:5050';
 
export async function GET() {
  try {
    const response = await fetch(`${API_BASE_URL}/results`);
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    return NextResponse.json(
      { error: 'Failed to fetch data' },
      { status: 500 }
    );
  }
}